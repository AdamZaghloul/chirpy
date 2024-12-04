package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	serveMux := http.NewServeMux()

	server := http.Server{
		Handler: serveMux,
		Addr:    ":8080",
	}

	serveMux.Handle("/app/", http.StripPrefix("/app", http.FileServer(http.Dir("."))))
	serveMux.Handle("/healthz", http.HandlerFunc(healthHandler))

	err := server.ListenAndServe()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		os.Exit(1)
	}
}
