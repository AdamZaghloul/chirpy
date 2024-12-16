package config

import (
	"chirpy/internal/auth"
	"chirpy/internal/database"
	"encoding/json"
	"log"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"
)

func (cfg *ApiConfig) ChirpsHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string    `json:"body"`
		User uuid.UUID `json:"user_id"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	token, err := auth.GetToken(r.Header, "Bearer ")
	if err != nil {
		log.Printf("error parsing token: %s", err)
		w.WriteHeader(401)
		return
	}

	user, err := auth.ValidateJWT(token, cfg.TokenSecret)
	if err != nil {
		log.Printf("invalid token: %s", err)
		w.WriteHeader(401)
		return
	}

	newChirp := database.CreateChirpParams{}

	if len(params.Body) <= 140 {
		newChirp.Body = cleanChirp(params.Body)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write([]byte("Chirp Too Long"))
		return
	}
	newChirp.UserID = user

	enteredChirp, err := cfg.Db.CreateChirp(r.Context(), newChirp)
	if err != nil {
		log.Printf("Error creating chirp: %s", err)
		w.WriteHeader(500)
		return
	}

	dat, err := json.Marshal(enteredChirp)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	w.Write(dat)
}

func (cfg *ApiConfig) GetChirpsHandler(w http.ResponseWriter, r *http.Request) {

	author := r.URL.Query().Get("author_id")
	sortMode := r.URL.Query().Get("sort")

	params := database.GetChirpsParams{}

	if author != "" {
		params.UserID = uuid.MustParse(author)
		params.Skip = false
	} else {
		params.Skip = true
	}

	if sortMode == "desc" {
		params.OrderBy = "desc"
	} else {
		params.OrderBy = "asc"
	}

	chirps, err := cfg.Db.GetChirps(r.Context(), params)
	if err != nil {
		log.Printf("Error retrieving chirps: %s", err)
		w.WriteHeader(500)
		return
	}

	dat, err := json.Marshal(chirps)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(dat)
}

func (cfg *ApiConfig) GetChirpHandler(w http.ResponseWriter, r *http.Request) {

	chirp, err := cfg.Db.GetChirp(r.Context(), uuid.MustParse(r.PathValue("chirpID")))
	if err != nil {
		log.Printf("Error retrieving chirp: %s", err)
		w.WriteHeader(404)
		return
	}

	dat, err := json.Marshal(chirp)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(dat)
}

func (cfg *ApiConfig) DeleteChirpHandler(w http.ResponseWriter, r *http.Request) {

	token, err := auth.GetToken(r.Header, "Bearer ")
	if err != nil {
		log.Printf("Error getting access token: %s", err)
		w.WriteHeader(401)
		return
	}

	user, err := auth.ValidateJWT(token, cfg.TokenSecret)
	if err != nil {
		log.Printf("Error validating access token: %s", err)
		w.WriteHeader(401)
		return
	}

	chirp, err := cfg.Db.GetChirp(r.Context(), uuid.MustParse(r.PathValue("chirpID")))
	if err != nil {
		log.Printf("Error retrieving chirp: %s", err)
		w.WriteHeader(404)
		return
	}

	if chirp.UserID != user {
		log.Printf("Requestor not owner of tweet: %s", err)
		w.WriteHeader(403)
	}

	deleteParams := database.DeleteChirpParams{
		UserID: user,
		ID:     chirp.ID,
	}
	err = cfg.Db.DeleteChirp(r.Context(), deleteParams)
	if err != nil {
		log.Printf("Error deleting chirp: %s", err)
		w.WriteHeader(500)
		return
	}

	w.WriteHeader(204)
}

func cleanChirp(chirp string) string {

	naughtyWords := []string{
		"kerfuffle",
		"sharbert",
		"fornax",
	}

	words := strings.Split(chirp, " ")

	for i, word := range words {
		if slices.Contains(naughtyWords, strings.ToLower(word)) {
			words[i] = "****"
		}
	}

	return strings.Join(words, " ")
}
