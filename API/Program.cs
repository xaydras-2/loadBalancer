using API.http;

namespace API
{
    // Program entry point
    public class Program
    {
        public static async Task Main(string[] args)
        {
            var server = new SimpleHttpServer("http://*:8080/");
            await server.Start();
        }
    }
}