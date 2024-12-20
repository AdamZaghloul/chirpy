package main

import (
	"chirpy/internal/config"
	"chirpy/internal/database"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		fmt.Println("error: no connection string")
		return
	}

	tokenSecret := os.Getenv("TOKEN_SECRET")
	if tokenSecret == "" {
		fmt.Println("error: no token secret")
		return
	}

	polkaKey := os.Getenv("POLKA_KEY")
	if polkaKey == "" {
		fmt.Println("error: no polka key")
		return
	}

	serveMux := http.NewServeMux()

	server := http.Server{
		Handler: serveMux,
		Addr:    ":8080",
	}
	fmt.Printf("Connecting with string: %s\n", dbURL)
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Println(err.Error())
	}

	defer db.Close()

	// Test the connection
	err = db.Ping()
	if err != nil {
		fmt.Printf("Error connecting to the database: %v\n", err)
		return
	}

	dbQueries := database.New(db)

	cfg := config.ApiConfig{
		FileserverHits: atomic.Int32{},
		Db:             *dbQueries,
		TokenSecret:    tokenSecret,
		PolkaKey:       polkaKey,
	}

	serveMux.Handle("/app/", http.StripPrefix("/app", cfg.MiddlewareMetricsInc(http.FileServer(http.Dir(".")))))

	serveMux.Handle("GET /api/healthz", http.HandlerFunc(config.HealthHandler))
	serveMux.Handle("GET /api/chirps", http.HandlerFunc(cfg.GetChirpsHandler))
	serveMux.Handle("GET /api/chirps/{chirpID}", http.HandlerFunc(cfg.GetChirpHandler))
	serveMux.Handle("DELETE /api/chirps/{chirpID}", http.HandlerFunc(cfg.DeleteChirpHandler))
	serveMux.Handle("POST /api/chirps", http.HandlerFunc(cfg.ChirpsHandler))
	serveMux.Handle("POST /api/users", http.HandlerFunc(cfg.UsersHandler))
	serveMux.Handle("PUT /api/users", http.HandlerFunc(cfg.UsersPutHandler))
	serveMux.Handle("POST /api/login", http.HandlerFunc(cfg.LoginHandler))
	serveMux.Handle("POST /api/refresh", http.HandlerFunc(cfg.RefreshHandler))
	serveMux.Handle("POST /api/revoke", http.HandlerFunc(cfg.RevokeHandler))
	serveMux.Handle("POST /api/polka/webhooks", http.HandlerFunc(cfg.ChirpyRedHandler))

	serveMux.Handle("GET /admin/metrics", http.HandlerFunc(cfg.MetricsHandler))
	serveMux.Handle("POST /admin/reset", http.HandlerFunc(cfg.ResetMetricsHandler))

	err = server.ListenAndServe()
	if err != nil {
		fmt.Println(err.Error())
	}

}
