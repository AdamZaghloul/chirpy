package main

import (
	"chirpy/internal/auth"
	"chirpy/internal/database"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             database.Queries
	tokenSecret    string
	polkaKey       string
}

func main() {
	godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	tokenSecret := os.Getenv("TOKEN_SECRET")
	polkaKey := os.Getenv("POLKA_KEY")

	serveMux := http.NewServeMux()

	server := http.Server{
		Handler: serveMux,
		Addr:    ":8080",
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Println(err.Error())
	}

	dbQueries := database.New(db)

	cfg := apiConfig{
		fileserverHits: atomic.Int32{},
		db:             *dbQueries,
		tokenSecret:    tokenSecret,
		polkaKey:       polkaKey,
	}

	serveMux.Handle("/app/", http.StripPrefix("/app", cfg.middlewareMetricsInc(http.FileServer(http.Dir(".")))))

	serveMux.Handle("GET /api/healthz", http.HandlerFunc(healthHandler))
	serveMux.Handle("GET /api/chirps", http.HandlerFunc(cfg.getChirpsHandler))
	serveMux.Handle("GET /api/chirps/{chirpID}", http.HandlerFunc(cfg.getChirpHandler))
	serveMux.Handle("DELETE /api/chirps/{chirpID}", http.HandlerFunc(cfg.deleteChirpHandler))
	serveMux.Handle("POST /api/chirps", http.HandlerFunc(cfg.chirpsHandler))
	serveMux.Handle("POST /api/users", http.HandlerFunc(cfg.usersHandler))
	serveMux.Handle("PUT /api/users", http.HandlerFunc(cfg.usersPutHandler))
	serveMux.Handle("POST /api/login", http.HandlerFunc(cfg.loginHandler))
	serveMux.Handle("POST /api/refresh", http.HandlerFunc(cfg.refreshHandler))
	serveMux.Handle("POST /api/revoke", http.HandlerFunc(cfg.revokeHandler))
	serveMux.Handle("POST /api/polka/webhooks", http.HandlerFunc(cfg.chirpyRedHandler))

	serveMux.Handle("GET /admin/metrics", http.HandlerFunc(cfg.metricsHandler))
	serveMux.Handle("POST /admin/reset", http.HandlerFunc(cfg.resetMetricsHandler))

	err = server.ListenAndServe()
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
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	_, err := w.Write([]byte(fmt.Sprintf(`
		<html>
			<body>
				<h1>Welcome, Chirpy Admin</h1>
				<p>Chirpy has been visited %d times!</p>
			</body>
		</html>`, cfg.fileserverHits.Load())))
	if err != nil {
		fmt.Println(err.Error())
	}
}

func (cfg *apiConfig) resetMetricsHandler(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("PLATFORM") == "dev" {
		cfg.fileserverHits.Store(0)
		cfg.db.Reset(r.Context())

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

func (cfg *apiConfig) usersHandler(w http.ResponseWriter, r *http.Request) {
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

	respBody, err := cfg.db.CreateUser(r.Context(), createUserParams)
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

func (cfg *apiConfig) usersPutHandler(w http.ResponseWriter, r *http.Request) {
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

	user, err := auth.ValidateJWT(token, cfg.tokenSecret)
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

	respBody, err := cfg.db.UpdateUser(r.Context(), updateUserParams)
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

func (cfg *apiConfig) loginHandler(w http.ResponseWriter, r *http.Request) {
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

	user, err := cfg.db.GetUser(r.Context(), params.Email)
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

	token, err := auth.MakeJWT(user.ID, cfg.tokenSecret, time.Duration(expiresIn)*time.Second)
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

	refreshToken, err := cfg.db.CreateRefreshToken(r.Context(), refreshTokenParams)
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
func (cfg *apiConfig) refreshHandler(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetToken(r.Header, "Bearer ")
	if err != nil {
		log.Printf("Error getting refresh token: %s", err)
		w.WriteHeader(401)
		return
	}

	user, err := cfg.db.GetUserFromRefreshToken(r.Context(), token)
	if err != nil {
		log.Printf("Error getting user by refresh token: %s", err)
		w.WriteHeader(401)
		return
	}

	accessToken, err := auth.MakeJWT(user.UUID, cfg.tokenSecret, time.Duration(3600))
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

func (cfg *apiConfig) revokeHandler(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetToken(r.Header, "Bearer ")
	if err != nil {
		log.Printf("Error getting refresh token: %s", err)
		w.WriteHeader(500)
		return
	}

	_, err = cfg.db.RevokeRefreshToken(r.Context(), token)
	if err != nil {
		log.Printf("Error revoking refresh token: %s", err)
		w.WriteHeader(500)
		return
	}

	w.WriteHeader(204)
}

func (cfg *apiConfig) chirpyRedHandler(w http.ResponseWriter, r *http.Request) {

	token, err := auth.GetToken(r.Header, "ApiKey ")
	if err != nil {
		log.Printf("error parsing API Key: %s", err)
		w.WriteHeader(401)
		return
	}

	if token != cfg.polkaKey {
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

	err = cfg.db.UpgradeChirpyRed(r.Context(), params.Data.UserId)
	if err != nil {
		log.Printf("Error upgrading user: %s", err)
		w.WriteHeader(404)
		return
	}

	w.WriteHeader(204)

}

func (cfg *apiConfig) chirpsHandler(w http.ResponseWriter, r *http.Request) {
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

	user, err := auth.ValidateJWT(token, cfg.tokenSecret)
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

	enteredChirp, err := cfg.db.CreateChirp(r.Context(), newChirp)
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

func (cfg *apiConfig) getChirpsHandler(w http.ResponseWriter, r *http.Request) {
	//var filteredChirps []database.Chirp

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

	chirps, err := cfg.db.GetChirps(r.Context(), params)
	if err != nil {
		log.Printf("Error retrieving chirps: %s", err)
		w.WriteHeader(500)
		return
	}

	/*if author != "" {
		for _, chirp := range chirps {
			if chirp.UserID == uuid.MustParse(author) {
				filteredChirps = append(filteredChirps, chirp)
			}
		}
	} else {
		filteredChirps = chirps
	}

	if sortMode == "desc" {
		sort.Slice(filteredChirps, func(i, j int) bool {
			timeI := filteredChirps[i].CreatedAt
			timeJ := filteredChirps[j].CreatedAt
			return timeI.After(timeJ) // Descending order
		})
	}*/

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

func (cfg *apiConfig) getChirpHandler(w http.ResponseWriter, r *http.Request) {

	chirp, err := cfg.db.GetChirp(r.Context(), uuid.MustParse(r.PathValue("chirpID")))
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

func (cfg *apiConfig) deleteChirpHandler(w http.ResponseWriter, r *http.Request) {

	token, err := auth.GetToken(r.Header, "Bearer ")
	if err != nil {
		log.Printf("Error getting access token: %s", err)
		w.WriteHeader(401)
		return
	}

	user, err := auth.ValidateJWT(token, cfg.tokenSecret)
	if err != nil {
		log.Printf("Error validating access token: %s", err)
		w.WriteHeader(401)
		return
	}

	chirp, err := cfg.db.GetChirp(r.Context(), uuid.MustParse(r.PathValue("chirpID")))
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
	err = cfg.db.DeleteChirp(r.Context(), deleteParams)
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
