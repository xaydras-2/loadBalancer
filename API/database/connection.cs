using Npgsql;

namespace API.database
{
    public static class ConnectionFactory
    {
        // I didn't use ".env var" since, in my case, I only created this API for test purposes, not for production.
        private const string ConnString =
            "Host=postgres_db;Port=5432;Username=postgres;Password=postgres2025;Database=test_lb;Pooling=true";
        
        private const int MaxRetryAttempts = 5;
        private const int BaseDelayMs = 1000; // 1 second

        /// <summary>
        /// Creates and opens a new NpgsqlConnection with retry logic. Caller is responsible for disposing it.
        /// </summary>
        public static NpgsqlConnection CreateOpenConnection()
        {
            var attempt = 0;
            
            while (attempt < MaxRetryAttempts)
            {
                try
                {
                    var conn = new NpgsqlConnection(ConnString);
                    conn.Open();
                    return conn;
                }
                catch (Exception ex) when (IsTransientError(ex))
                {
                    attempt++;
                    
                    if (attempt >= MaxRetryAttempts)
                    {
                        throw new InvalidOperationException(
                            $"Failed to connect to database after {MaxRetryAttempts} attempts. Last error: {ex.Message}", 
                            ex);
                    }
                    
                    var delay = CalculateDelay(attempt);
                    Console.WriteLine($"Database connection attempt {attempt} failed. Retrying in {delay}ms. Error: {ex.Message}");
                    Thread.Sleep(delay);
                }
            }
            
            throw new InvalidOperationException("Unexpected exit from retry loop");
        }

        /// <summary>
        /// Async version for high‑throughput or UI apps with retry logic.
        /// </summary>
        public static async Task<NpgsqlConnection> CreateOpenConnectionAsync()
        {
            var attempt = 0;
            
            while (attempt < MaxRetryAttempts)
            {
                try
                {
                    var conn = new NpgsqlConnection(ConnString);
                    await conn.OpenAsync();
                    return conn;
                }
                catch (Exception ex) when (IsTransientError(ex))
                {
                    attempt++;
                    
                    if (attempt >= MaxRetryAttempts)
                    {
                        throw new InvalidOperationException(
                            $"Failed to connect to database after {MaxRetryAttempts} attempts. Last error: {ex.Message}", 
                            ex);
                    }
                    
                    var delay = CalculateDelay(attempt);
                    Console.WriteLine($"Database connection attempt {attempt} failed. Retrying in {delay}ms. Error: {ex.Message}");
                    await Task.Delay(delay);
                }
            }
            
            throw new InvalidOperationException("Unexpected exit from retry loop");
        }

        /// <summary>
        /// Determines if an exception is a transient error that should be retried.
        /// </summary>
        private static bool IsTransientError(Exception ex)
        {
            return ex switch
            {
                // Network-related errors
                NpgsqlException npgsqlEx when npgsqlEx.Message.Contains("Connection refused") => true,
                NpgsqlException npgsqlEx when npgsqlEx.Message.Contains("timeout") => true,
                NpgsqlException npgsqlEx when npgsqlEx.Message.Contains("No route to host") => true,
                NpgsqlException npgsqlEx when npgsqlEx.Message.Contains("Network is unreachable") => true,
                NpgsqlException npgsqlEx when npgsqlEx.Message.Contains("Failed to connect to") => true,
                
                // General network exceptions
                System.Net.Sockets.SocketException => true,
                TimeoutException => true,
                
                // PostgreSQL server not ready
                NpgsqlException npgsqlEx when npgsqlEx.Message.Contains("the database system is starting up") => true,
                NpgsqlException npgsqlEx when npgsqlEx.Message.Contains("server is not accepting connections") => true,
                
                // Don't retry authentication or configuration errors
                NpgsqlException npgsqlEx when npgsqlEx.Message.Contains("authentication failed") => false,
                NpgsqlException npgsqlEx when npgsqlEx.Message.Contains("database") && npgsqlEx.Message.Contains("does not exist") => false,
                
                _ => false
            };
        }

        /// <summary>
        /// Calculates exponential backoff delay with jitter.
        /// </summary>
        private static int CalculateDelay(int attempt)
        {
            var exponentialDelay = BaseDelayMs * Math.Pow(2, attempt - 1);
            var maxDelay = Math.Min(exponentialDelay, 30000); // Cap at 30 seconds
            
            // Add some jitter to prevent thundering herd
            var random = new Random();
            var jitter = random.Next(0, (int)(maxDelay * 0.1));
            
            return (int)(maxDelay + jitter);
        }
    }
}