using API.database.repository;
using API.http.Models;
using System.Text.Json;
using API.database.migration;

namespace API
{
    public class Program
    {
        public static void Main(string[] args)
        {

            var builder = WebApplication.CreateBuilder(args);
            var jsonSerOpt = new JsonSerializerOptions { PropertyNameCaseInsensitive = true };

            // Add services to the container
            builder.Services.AddControllers();
            builder.Services.AddCors(options =>
            {
                options.AddDefaultPolicy(policy =>
                {
                    policy.AllowAnyOrigin()
                          .AllowAnyMethod()
                          .AllowAnyHeader();
                });
            });

            // Configure JSON serialization
            builder.Services.Configure<JsonSerializerOptions>(options =>
            {
                options.PropertyNameCaseInsensitive = true;
            });

            var app = builder.Build();

            // Configure the HTTP request pipeline
            app.UseCors();
            app.UseRouting();

            // Health check endpoint
            app.MapGet("/healthz", () => Results.Json(new { status = "ok" }));

            // User endpoints
            app.MapGet("/api/users", () =>
            {
                Console.WriteLine("GetAllUsers has been called");
                var users = UserRepository.All();
                if (users == null)
                {
                    return Results.Json(new { error = "Can't return all the users" });
                }
                return Results.Json(users);
            });

            app.MapGet("/api/users/{id:int}", (int id) =>
            {
                var user = UserRepository.GetById(id);
                if (user == null)
                {
                    return Results.NotFound(new { error = "User not found" });
                }
                Console.WriteLine("GetUser has been called");
                return Results.Json(user);
            });

            app.MapPost("/api/users", async (HttpRequest request) =>
            {
                try
                {
                    using var reader = new StreamReader(request.Body);
                    var requestBody = await reader.ReadToEndAsync();

                    var newUser = JsonSerializer.Deserialize<User>(requestBody, jsonSerOpt);
                    if (newUser == null)
                    {
                        return Results.BadRequest(new { error = "The new user is null" });
                    }
                    var createdUser = UserRepository.Add(newUser);
                    if (createdUser == null)
                    {
                        return Results.BadRequest(new { error = "Failed to create user" });
                    }

                    Console.WriteLine("CreateUser has been called");
                    return Results.Json(createdUser);
                }
                catch (Exception ex)
                {
                    return Results.BadRequest(new { error = "Invalid user data", details = ex.Message });
                }
            });

            app.MapPut("/api/users/{id:int}", async (int id, HttpRequest request) =>
            {
                try
                {
                    var user = UserRepository.GetById(id);
                    if (user == null)
                    {
                        return Results.NotFound(new { error = "User not found" });
                    }

                    using var reader = new StreamReader(request.Body);
                    var requestBody = await reader.ReadToEndAsync();

                    var updatedUser = JsonSerializer.Deserialize<User>(requestBody, jsonSerOpt);
                    if (updatedUser == null)
                    {
                        var errorPayload = new
                        {
                            error = "Null when updating the user",
                            userId = id,
                            time = DateTime.UtcNow
                        };
                        return Results.BadRequest(errorPayload);
                    }
                    user.Name = updatedUser.Name ?? user.Name;
                    user.Email = updatedUser.Email ?? user.Email;
                    UserRepository.Update(user);
                    Console.WriteLine("UpdateUser has been called");
                    return Results.Json(user);
                }
                catch (Exception ex)
                {
                    return Results.BadRequest(new { error = "Invalid user data", details = ex.Message });
                }
            });

            app.MapDelete("/api/users/{id:int}", (int id) =>
            {
                var user = UserRepository.GetById(id);
                if (user == null)
                {
                    return Results.NotFound(new { error = "User not found" });
                }

                UserRepository.Delete(id);
                Console.WriteLine("DeleteUser has been called");
                return Results.Json(new { message = "User deleted successfully" });
            });

            // Configure to listen on all interfaces
            app.Urls.Add("http://0.0.0.0:8080");

            Console.WriteLine("Server starting on http://0.0.0.0:8080");
            Console.WriteLine("API Endpoints:");
            Console.WriteLine("GET    /api/users     - Get all users");
            Console.WriteLine("GET    /api/users/{id} - Get user by ID");
            Console.WriteLine("POST   /api/users     - Create new user");
            Console.WriteLine("PUT    /api/users/{id} - Update user");
            Console.WriteLine("DELETE /api/users/{id} - Delete user");
            Console.WriteLine("\nPress Ctrl+C to stop...");


            // Run migration just before starting the server
            try
            {
                Console.WriteLine("Running database migration...");
                CreateUsersTableMigration.Migrate();
                Console.WriteLine("Database migration completed successfully.");
            }
            catch (Exception ex)
            {
                Console.WriteLine($"Database migration failed: {ex.Message}");
            }

            app.Run();
        }
    }
}