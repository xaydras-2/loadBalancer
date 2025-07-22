using Npgsql;

namespace API.database.migration
{
    /// <summary>
    /// Simple migration to create the "users" table if it does not already exist.
    /// Call this at application startup before using the repository.
    /// </summary>
    public static class CreateUsersTableMigration
    {
        public static void Migrate()
        {
            // Check if the "users" table already exists
            const string checkTableSql = @"
                SELECT EXISTS (
                    SELECT 1
                    FROM information_schema.tables
                    WHERE table_schema = 'public'
                      AND table_name = 'users'
                );";

            using var conn = ConnectionFactory.CreateOpenConnection();
            using (var checkCmd = new NpgsqlCommand(checkTableSql, conn))
            {
                var result = checkCmd.ExecuteScalar();
                bool exists = result != null && Convert.ToBoolean(result);
                if (exists)
                {
                    Console.WriteLine("[Migration] 'users' table already exists, skipping migration.");
                    return;
                }
            }

            // Create the table since it does not exist
            const string createTableSql = @"
                CREATE TABLE public.users (
                    id SERIAL PRIMARY KEY,
                    name VARCHAR(100) NOT NULL,
                    email VARCHAR(200) NOT NULL UNIQUE,
                    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
                );";

            using var createCmd = new NpgsqlCommand(createTableSql, conn);

            try
            {
                createCmd.ExecuteNonQuery();
                Console.WriteLine("[Migration] 'users' table created successfully.");
            }
            catch (Exception ex)
            {
                Console.Error.WriteLine($"[Migration] Failed to create 'users' table: {ex.Message}");
                throw;
            }
        }
    }
}
