using Npgsql;
using API.http.Models;

namespace API.database.repository
{
    public class UserRepository
    {
        /// <summary>
        /// Retrieves all users from the database.
        /// </summary>
        /// <returns>
        /// A list of <see cref="User"/> instances representing all users.
        /// </returns>
        public static List<User> All()
        {
            var users = new List<User>();

            using var conn = ConnectionFactory.CreateOpenConnection();
            using var cmd = new NpgsqlCommand("SELECT id, name, email FROM users", conn);
            using var reader = cmd.ExecuteReader();

            while (reader.Read())
            {
                users.Add(new User
                {
                    Id = reader.GetInt32(0),
                    Name = reader.GetString(1),
                    Email = reader.GetString(2)
                });
            }

            return users;
        }

        /// <summary>
        /// Retrieves a single user by their unique identifier.
        /// </summary>
        /// <param name="id">The unique identifier of the user to retrieve.</param>
        /// <returns>
        /// A <see cref="User"/> instance if found; otherwise <c>null</c>.
        /// </returns>
        public static User? GetById(int id)
        {
            using var conn = ConnectionFactory.CreateOpenConnection();
            using var cmd = new NpgsqlCommand("SELECT id, name, email FROM users WHERE id = @id", conn);
            cmd.Parameters.AddWithValue("id", id);

            using var reader = cmd.ExecuteReader();

            if (!reader.Read()) return null;

            return new User
            {
                Id = reader.GetInt32(0),
                Name = reader.GetString(1),
                Email = reader.GetString(2)
            };
        }

        /// <summary>
        /// Inserts a new user into the database.
        /// </summary>
        /// <param name="user">The <see cref="User"/> object containing name and email.</param>
        /// <returns>
        /// A <see cref="User"/> instance if the insertion succeeds; otherwise <c>null</c>.
        /// The returned object's Id will match the generated Id from the database.
        /// </returns>
        public static User? Add(User user)
        {
            using var conn = ConnectionFactory.CreateOpenConnection();
            using var cmd = new NpgsqlCommand(
                "INSERT INTO users (name, email) VALUES (@name, @email) RETURNING id",
                conn
            );

            var userParams = new[]
            {
                new NpgsqlParameter("name", user.Name ?? (object)DBNull.Value),
                new NpgsqlParameter("email", user.Email ?? (object)DBNull.Value)
            };

            cmd.Parameters.AddRange(userParams);

            try
            {
                var newId = cmd.ExecuteScalar();
                if (newId != null && newId != DBNull.Value)
                {
                    return new User 
                    { 
                        Id = Convert.ToInt32(newId), 
                        Name = user.Name, 
                        Email = user.Email 
                    };
                }
            }
            catch (Exception ex)
            {
                Console.WriteLine($"Error adding user: {ex.Message}");
            }

            return null;
        }

        /// <summary>
        /// Updates an existing user's name and email based on their identifier.
        /// </summary>
        /// <param name="user">The <see cref="User"/> object containing updated data. Its Id must match an existing record.</param>
        /// <returns>
        /// A <see cref="User"/> instance if the update succeeds; otherwise <c>null</c>.
        /// </returns>
        public static User? Update(User user)
        {
            using var conn = ConnectionFactory.CreateOpenConnection();
            using var cmd = new NpgsqlCommand("UPDATE users SET name = @name, email = @email WHERE id = @id", conn);

            var userParams = new[]
            {
                new NpgsqlParameter("id", user.Id),
                new NpgsqlParameter("name", user.Name ?? (object)DBNull.Value),
                new NpgsqlParameter("email", user.Email ?? (object)DBNull.Value)
            };
            cmd.Parameters.AddRange(userParams);

            try
            {
                var rows = cmd.ExecuteNonQuery();
                return rows > 0
                    ? new User { Id = user.Id, Name = user.Name, Email = user.Email }
                    : null;
            }
            catch (Exception ex)
            {
                Console.WriteLine($"Error updating user: {ex.Message}");
                return null;
            }
        }

        /// <summary>
        /// Deletes a user from the database by identifier.
        /// </summary>
        /// <param name="id">The unique identifier of the user to delete.</param>
        /// <returns>
        /// <c>true</c> if deletion succeeds; otherwise <c>false</c>.
        /// </returns>
        public static bool Delete(int id)
        {
            using var conn = ConnectionFactory.CreateOpenConnection();
            using var cmd = new NpgsqlCommand("DELETE FROM users WHERE id = @id", conn);
            cmd.Parameters.AddWithValue("id", id);

            try
            {
                var rows = cmd.ExecuteNonQuery();
                return rows > 0;
            }
            catch (Exception ex)
            {
                Console.WriteLine($"Error deleting user: {ex.Message}");
                return false;
            }
        }
    }
}