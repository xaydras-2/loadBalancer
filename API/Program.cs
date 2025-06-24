using System;
using System.Collections.Generic;
using System.IO;
using System.Net;
using System.Text;
using System.Text.Json;
using System.Threading.Tasks;

// Simple data model
public class User
{
    public int Id { get; set; }
    public string? Name { get; set; }
    public string? Email { get; set; }
}

// Simple API Controller
public class ApiController
{
    private readonly List<User> users;

    public ApiController()
    {
        // Initialize with some sample data
        users = new List<User>
        {
            new User { Id = 1, Name = "John Doe", Email = "john@example.com" },
            new User { Id = 2, Name = "Jane Smith", Email = "jane@example.com" }
        };
    }

    // Handle HTTP requests
    public async Task HandleRequest(HttpListenerContext context)
    {
        var request = context.Request;
        var response = context.Response;

        try
        {
            // Set CORS headers
            response.Headers.Add("Access-Control-Allow-Origin", "*");
            response.Headers.Add("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS");
            response.Headers.Add("Access-Control-Allow-Headers", "Content-Type");

            // Handle preflight OPTIONS request
            if (request.HttpMethod == "OPTIONS")
            {
                response.StatusCode = 200;
                response.Close();
                return;
            }

            string path = request.Url.AbsolutePath;
            string method = request.HttpMethod;

            string responseText = "";
            response.ContentType = "application/json";

            if (method == "GET" && path == "/healthz")
            {
                // Immediately return 200 OK with a tiny payload
                response.StatusCode = 200;
                response.ContentType = "application/json";
                var healthy = JsonSerializer.Serialize(new { status = "ok" });
                var data = Encoding.UTF8.GetBytes(healthy);
                response.ContentLength64 = data.Length;
                await response.OutputStream.WriteAsync(data, 0, data.Length);
                response.Close();
                return;
            }

            // Route requests
            switch ($"{method} {path}")
            {
                case "GET /api/users":
                    responseText = GetAllUsers();
                    break;

                case "GET /api/users/{id}":
                    int userId = ExtractIdFromPath(path);
                    responseText = GetUser(userId);
                    break;

                case "POST /api/users":
                    string requestBody = await ReadRequestBody(request);
                    responseText = CreateUser(requestBody);
                    break;

                case "PUT /api/users/{id}":
                    int updateId = ExtractIdFromPath(path);
                    string updateBody = await ReadRequestBody(request);
                    responseText = UpdateUser(updateId, updateBody);
                    break;

                case "DELETE /api/users/{id}":
                    int deleteId = ExtractIdFromPath(path);
                    responseText = DeleteUser(deleteId);
                    break;

                default:
                    response.StatusCode = 404;
                    responseText = JsonSerializer.Serialize(new { error = "Endpoint not found" });
                    break;
            }

            // Send response
            byte[] buffer = Encoding.UTF8.GetBytes(responseText);
            response.ContentLength64 = buffer.Length;
            await response.OutputStream.WriteAsync(buffer, 0, buffer.Length);
        }
        catch (Exception ex)
        {
            response.StatusCode = 500;
            string errorResponse = JsonSerializer.Serialize(new { error = ex.Message });
            byte[] errorBuffer = Encoding.UTF8.GetBytes(errorResponse);
            response.ContentLength64 = errorBuffer.Length;
            await response.OutputStream.WriteAsync(errorBuffer, 0, errorBuffer.Length);
        }
        finally
        {
            response.Close();
        }
    }

    // API Methods
    private string GetAllUsers()
    {
        Console.WriteLine("GetAllUsers has been called");
        return JsonSerializer.Serialize(users);
    }

    private string GetUser(int id)
    {
        var user = users.Find(u => u.Id == id);
        if (user == null)
        {
            return JsonSerializer.Serialize(new { error = "User not found" });
        }
        Console.WriteLine("GetUser has been called");
        return JsonSerializer.Serialize(user);
    }

    private string CreateUser(string requestBody)
    {
        try
        {
            var newUser = JsonSerializer.Deserialize<User>(requestBody);
            newUser.Id = users.Count > 0 ? users.Max(u => u.Id) + 1 : 1;
            users.Add(newUser);
            Console.WriteLine("CreateUser has been called");
            return JsonSerializer.Serialize(newUser);
        }
        catch
        {
            return JsonSerializer.Serialize(new { error = "Invalid user data" });
        }
    }

    private string UpdateUser(int id, string requestBody)
    {
        try
        {
            var user = users.Find(u => u.Id == id);
            if (user == null)
            {
                return JsonSerializer.Serialize(new { error = "User not found" });
            }

            var updatedUser = JsonSerializer.Deserialize<User>(requestBody);
            user.Name = updatedUser.Name ?? user.Name;
            user.Email = updatedUser.Email ?? user.Email;
            Console.WriteLine("UpdateUser has been called");
            return JsonSerializer.Serialize(user);
        }
        catch
        {
            return JsonSerializer.Serialize(new { error = "Invalid user data" });
        }
    }

    private string DeleteUser(int id)
    {
        var user = users.Find(u => u.Id == id);
        if (user == null)
        {
            return JsonSerializer.Serialize(new { error = "User not found" });
        }

        users.Remove(user);
        Console.WriteLine("DeleteUser has been called");
        return JsonSerializer.Serialize(new { message = "User deleted successfully" });
    }

    // Helper methods
    private int ExtractIdFromPath(string path)
    {
        string[] segments = path.Split('/');
        if (segments.Length >= 4 && int.TryParse(segments[3], out int id))
        {
            return id;
        }
        return 0;
    }

    private async Task<string> ReadRequestBody(HttpListenerRequest request)
    {
        using (var reader = new StreamReader(request.InputStream, request.ContentEncoding))
        {
            return await reader.ReadToEndAsync();
        }
    }
}

// HTTP Server
public class SimpleHttpServer
{
    private readonly HttpListener listener;
    private readonly ApiController controller;

    public SimpleHttpServer(string prefix)
    {
        listener = new HttpListener();
        listener.Prefixes.Add(prefix);
        controller = new ApiController();
    }

    public async Task Start()
    {
        listener.Start();
        Console.WriteLine($"Listener {string.Join(", ", listener.Prefixes)}");
        Console.WriteLine($"Server started on {listener.Prefixes.First()}");
        Console.WriteLine("API Endpoints:");
        Console.WriteLine("GET    /api/users     - Get all users");
        Console.WriteLine("GET    /api/users/{id} - Get user by ID");
        Console.WriteLine("POST   /api/users     - Create new user");
        Console.WriteLine("PUT    /api/users/{id} - Update user");
        Console.WriteLine("DELETE /api/users/{id} - Delete user");
        Console.WriteLine("\nPress 'q' to quit...");

        while (true)
        {
            var context = await listener.GetContextAsync();
            _ = Task.Run(() => controller.HandleRequest(context));
        }

        // listener.Stop();
    }
}

// Program entry point
public class Program
{
    public static async Task Main(string[] args)
    {
        var server = new SimpleHttpServer("http://*:8080/");
        await server.Start();
    }
}