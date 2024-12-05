package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func main() {
	serveMux := http.NewServeMux()

	server := http.Server{
		Handler: serveMux,
		Addr:    ":8080",
	}

	cfg := apiConfig{
		fileserverHits: atomic.Int32{},
	}

	serveMux.Handle("/app/", http.StripPrefix("/app", cfg.middlewareMetricsInc(http.FileServer(http.Dir(".")))))
	serveMux.Handle("/healthz", http.HandlerFunc(healthHandler))
	serveMux.Handle("/metrics", http.HandlerFunc(cfg.metricsHandler))
	serveMux.Handle("/reset", http.HandlerFunc(cfg.resetMetricsHandler))

	err := server.ListenAndServe()
	if err != nil {
		fmt.Println(err.Error())
	}

}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		fmt.Println(err.Error())
	}
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	_, err := w.Write([]byte(fmt.Sprintf("Hits: %d", cfg.fileserverHits.Load())))
	if err != nil {
		fmt.Println(err.Error())
	}
}

func (cfg *apiConfig) resetMetricsHandler(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)

	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		fmt.Println(err.Error())
	}
}
