# chirpy
Backend for Chirpy: a social media app like Twitter.

* Stores users and chirps (tweets) in a Postgres database, hashing passwords for security
* User authentication and authorization using JWT access tokens and refresh tokens
* Webhook payment processor integration

Note that you'll need Go, Postgres, Goose and SQLC installed to run the program.

To install:
1. Clone the repo locally.
2. Navigate to the chirpy directory
3. Run sqlc generate to generate required objects
4. Create a postgres database to use for the project and run "goose postgres "CONNECTION_STRING" up" until you are up to the latest database migration
4. Run "go run ."
6. You can now send HTTP requests to the server. See main.go for endpoints.
