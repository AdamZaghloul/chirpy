package config

import (
	"fmt"
	"net/http"
	"os"
)

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		fmt.Println(err.Error())
	}
}

func (cfg *ApiConfig) MiddlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.FileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *ApiConfig) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	_, err := w.Write([]byte(fmt.Sprintf(`
		<html>
			<body>
				<h1>Welcome, Chirpy Admin</h1>
				<p>Chirpy has been visited %d times!</p>
			</body>
		</html>`, cfg.FileserverHits.Load())))
	if err != nil {
		fmt.Println(err.Error())
	}
}

func (cfg *ApiConfig) ResetMetricsHandler(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("PLATFORM") == "dev" {
		cfg.FileserverHits.Store(0)
		cfg.Db.Reset(r.Context())

		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("OK"))
		if err != nil {
			fmt.Println(err.Error())
		}
	} else {
		w.WriteHeader(http.StatusForbidden)
		_, err := w.Write([]byte("FORBIDDEN"))
		if err != nil {
			fmt.Println(err.Error())
		}
	}
}
