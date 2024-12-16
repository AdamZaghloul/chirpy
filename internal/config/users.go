package config

import (
	"chirpy/internal/auth"
	"chirpy/internal/database"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

func (cfg *ApiConfig) UsersHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	type returnVals struct {
		ID          uuid.UUID `json:"id"`
		Email       string    `json:"email"`
		Created_at  time.Time `json:"created_at"`
		Updated_at  time.Time `json:"updated_at"`
		IsChirpyRed bool      `json:"is_chirpy_red"`
	}

	hashedPass, err := auth.HashPassword(params.Password)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	createUserParams := database.CreateUserParams{
		Email:          params.Email,
		HashedPassword: hashedPass,
	}

	respBody, err := cfg.Db.CreateUser(r.Context(), createUserParams)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	respStruct := returnVals{
		ID:          respBody.ID,
		Email:       respBody.Email,
		Created_at:  respBody.CreatedAt,
		Updated_at:  respBody.UpdatedAt,
		IsChirpyRed: respBody.IsChirpyRed,
	}

	dat, err := json.Marshal(respStruct)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	w.Write(dat)
}

func (cfg *ApiConfig) UsersPutHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
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

	type returnVals struct {
		ID          uuid.UUID `json:"id"`
		Email       string    `json:"email"`
		Created_at  time.Time `json:"created_at"`
		Updated_at  time.Time `json:"updated_at"`
		IsChirpyRed bool      `json:"is_chirpy_red"`
	}

	hashedPass, err := auth.HashPassword(params.Password)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	updateUserParams := database.UpdateUserParams{
		Email:          params.Email,
		HashedPassword: hashedPass,
		ID:             user,
	}

	respBody, err := cfg.Db.UpdateUser(r.Context(), updateUserParams)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	respStruct := returnVals{
		ID:          respBody.ID,
		Email:       respBody.Email,
		Created_at:  respBody.CreatedAt,
		Updated_at:  respBody.UpdatedAt,
		IsChirpyRed: respBody.IsChirpyRed,
	}

	dat, err := json.Marshal(respStruct)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(dat)
}

func (cfg *ApiConfig) ChirpyRedHandler(w http.ResponseWriter, r *http.Request) {

	token, err := auth.GetToken(r.Header, "ApiKey ")
	if err != nil {
		log.Printf("error parsing API Key: %s", err)
		w.WriteHeader(401)
		return
	}

	if token != cfg.PolkaKey {
		log.Printf("Invalid API Key: %s", err)
		w.WriteHeader(401)
		return
	}

	type data struct {
		UserId uuid.UUID `json:"user_id"`
	}

	type parameters struct {
		Event string `json:"event"`
		Data  data   `json:"data"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	if params.Event != "user.upgraded" {
		log.Printf("Invalid polka event: %s", err)
		w.WriteHeader(204)
		return
	}

	err = cfg.Db.UpgradeChirpyRed(r.Context(), params.Data.UserId)
	if err != nil {
		log.Printf("Error upgrading user: %s", err)
		w.WriteHeader(404)
		return
	}

	w.WriteHeader(204)

}
