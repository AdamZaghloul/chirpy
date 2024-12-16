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

func (cfg *ApiConfig) LoginHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Expires  int    `json:"expires_in_seconds"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	user, err := cfg.Db.GetUser(r.Context(), params.Email)
	if err != nil {
		log.Printf("User not found: %s", err)
		w.WriteHeader(401)
		return
	}

	err = auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if err != nil {
		log.Printf("Wrong login or password: %s", err)
		w.WriteHeader(401)
		return
	}

	expiresIn := 0

	if params.Expires == 0 || params.Expires > (3600) {
		expiresIn = 3600
	} else {
		expiresIn = params.Expires
	}

	token, err := auth.MakeJWT(user.ID, cfg.TokenSecret, time.Duration(expiresIn)*time.Second)
	if err != nil {
		log.Printf("Error generating access token: %s", err)
		w.WriteHeader(500)
		return
	}

	rawRefreshToken, err := auth.MakeRefreshToken()
	if err != nil {
		log.Printf("Error generating refresh token: %s", err)
		w.WriteHeader(500)
		return
	}

	refreshTokenParams := database.CreateRefreshTokenParams{
		Token:  rawRefreshToken,
		UserID: user.ID,
	}

	refreshToken, err := cfg.Db.CreateRefreshToken(r.Context(), refreshTokenParams)
	if err != nil {
		log.Printf("Error inserting refresh token: %s", err)
		w.WriteHeader(500)
		return
	}

	type returnVals struct {
		ID           uuid.UUID `json:"id"`
		Email        string    `json:"email"`
		Created_at   time.Time `json:"created_at"`
		Updated_at   time.Time `json:"updated_at"`
		Token        string    `json:"token"`
		RefreshToken string    `json:"refresh_token"`
		IsChirpyRed  bool      `json:"is_chirpy_red"`
	}

	respStruct := returnVals{
		ID:           user.ID,
		Email:        user.Email,
		Created_at:   user.CreatedAt,
		Updated_at:   user.UpdatedAt,
		Token:        token,
		RefreshToken: refreshToken.Token,
		IsChirpyRed:  user.IsChirpyRed,
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
func (cfg *ApiConfig) RefreshHandler(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetToken(r.Header, "Bearer ")
	if err != nil {
		log.Printf("Error getting refresh token: %s", err)
		w.WriteHeader(401)
		return
	}

	user, err := cfg.Db.GetUserFromRefreshToken(r.Context(), token)
	if err != nil {
		log.Printf("Error getting user by refresh token: %s", err)
		w.WriteHeader(401)
		return
	}

	accessToken, err := auth.MakeJWT(user.UUID, cfg.TokenSecret, time.Duration(3600))
	if err != nil {
		log.Printf("Error generating access token: %s", err)
		w.WriteHeader(500)
		return
	}

	type returnVals struct {
		Token string `json:"token"`
	}

	respStruct := returnVals{
		Token: accessToken,
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

func (cfg *ApiConfig) RevokeHandler(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetToken(r.Header, "Bearer ")
	if err != nil {
		log.Printf("Error getting refresh token: %s", err)
		w.WriteHeader(500)
		return
	}

	_, err = cfg.Db.RevokeRefreshToken(r.Context(), token)
	if err != nil {
		log.Printf("Error revoking refresh token: %s", err)
		w.WriteHeader(500)
		return
	}

	w.WriteHeader(204)
}
