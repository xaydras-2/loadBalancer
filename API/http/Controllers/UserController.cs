using System.Net;
using System.Text;
using System.Text.Json;
using API.http.Models;
using API.database.repository;

namespace API.http.Controllers
{
    public class ApiController
    {
        private readonly List<User>? users;
        private static readonly JsonSerializerOptions JsonOpts = new JsonSerializerOptions { PropertyNameCaseInsensitive = true };



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

                string? path = request.Url.AbsolutePath;
                string method = request.HttpMethod;

                string responseText = "";
                response.ContentType = "application/json";


                switch (method)
                {
                    case "GET" when path == "/healthz":
                        {
                            // Immediately return 200 OK with a tiny payload
                            response.StatusCode = 200;
                            response.ContentType = "application/json";
                            var healthy = JsonSerializer.Serialize(new { status = "ok" });
                            var data = Encoding.UTF8.GetBytes(healthy);
                            response.ContentLength64 = data.Length;
                            await response.OutputStream.WriteAsync(data.AsMemory(), CancellationToken.None);
                            response.Close();
                        }
                        break;
                    case "GET" when path.StartsWith("/api/users/"):
                        {
                            int userId = ExtractIdFromPath(path);
                            responseText = GetUser(userId);
                        }
                        break;

                    case "PUT" when path.StartsWith("/api/users/"):
                        {
                            int userId = ExtractIdFromPath(path);
                            var body = await ReadRequestBody(request);
                            responseText = UpdateUser(userId, body);
                        }
                        break;

                    case "DELETE" when path.StartsWith("/api/users/"):
                        {
                            int userId = ExtractIdFromPath(path);
                            responseText = DeleteUser(userId);
                        }
                        break;

                    case "GET" when path == "/api/users":
                        responseText = GetAllUsers();
                        break;

                    case "POST" when path == "/api/users":
                        {
                            var body = await ReadRequestBody(request);
                            responseText = CreateUser(body);
                        }
                        break;

                    default:
                        response.StatusCode = 404;
                        responseText = JsonSerializer.Serialize(new { error = "Endpoint not found" });
                        break;
                }


                // Send response
                byte[] buffer = Encoding.UTF8.GetBytes(responseText);
                response.ContentLength64 = buffer.Length;
                await response.OutputStream.WriteAsync(buffer.AsMemory(), CancellationToken.None);
            }
            catch (Exception ex)
            {
                response.StatusCode = 500;
                string errorResponse = JsonSerializer.Serialize(new { error = ex.Message });
                byte[] errorBuffer = Encoding.UTF8.GetBytes(errorResponse);
                response.ContentLength64 = errorBuffer.Length;
                await response.OutputStream.WriteAsync(errorBuffer.AsMemory(), CancellationToken.None);
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
            var users = UserRepository.All();
            if (users == null)
            {
                return JsonSerializer.Serialize("Can't return all the users");
            }
            return JsonSerializer.Serialize(users);
        }

        private string GetUser(int id)
        {
            var user = UserRepository.GetById(id);
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
                var newUser = JsonSerializer.Deserialize<User>(requestBody, JsonOpts);
                if (newUser == null)
                {
                    return JsonSerializer.Serialize("error the new user is null");
                }
                UserRepository.Add(newUser);
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
                var user = UserRepository.GetById(id);
                if (user == null)
                {
                    return JsonSerializer.Serialize(new { error = "User not found" });
                }

                var updatedUser = JsonSerializer.Deserialize<User>(requestBody, JsonOpts);
                if (updatedUser == null)
                {
                    var errorPayload = new
                    {
                        error = "Null when updating the user",
                        userId = id,
                        time = DateTime.UtcNow
                    };
                    return JsonSerializer.Serialize(errorPayload);
                }
                user.Name = updatedUser.Name ?? user.Name;
                user.Email = updatedUser.Email ?? user.Email;
                UserRepository.Update(user);
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
            var user = UserRepository.GetById(id);
            if (user == null)
            {
                return JsonSerializer.Serialize(new { error = "User not found" });
            }

            UserRepository.Delete(id);
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

}