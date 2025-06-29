using System.Net;
using API.http.Controllers;

// HTTP Server
namespace API.http
{
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

}