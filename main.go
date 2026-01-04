package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jaidev-me/chirpy/internal/auth"
	"github.com/jaidev-me/chirpy/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
	jwtSecret      string
	polkaApiKey    string
}

const devMode = "dev"

// helpers

func responseWithError(w http.ResponseWriter, err string, statusCode int) {
	type resError struct {
		Error string `json:"error"`
	}

	w.Header().Set("Content-Type", "application/json")

	errorBody, marshalError := json.Marshal(resError{Error: err})
	if marshalError != nil {
		fmt.Printf("Error while parsing json: %v", marshalError)
		return
	}

	w.WriteHeader(statusCode)
	w.Write(errorBody)

}

func responseWithJson[T any](w http.ResponseWriter, data T, statusCode int) {
	w.Header().Set("Content-Type", "application/json")

	jsonData, marshalError := json.Marshal(data)

	if marshalError != nil {
		fmt.Printf("Error while parsing json: %v", marshalError)
		responseWithError(w, "Server Error", 500)
		return
	}

	// fmt.Println(string(jsonData))
	w.WriteHeader(statusCode)
	w.Write(jsonData)

}

func jsonDecoder[T any](w http.ResponseWriter, r *http.Request, store *T) {
	decoder := json.NewDecoder(r.Body)

	defer r.Body.Close()

	err := decoder.Decode(store)

	if err != nil {
		responseWithError(w, "Invalid request data!", 400)
	}

}

// handlers

func handlerHealthz(w http.ResponseWriter, _ *http.Request) {
	header := w.Header()
	header["Content-Type"] = []string{"text/plain; charset=utf-8"}
	w.WriteHeader(200)
	res := "OK"
	code, err := w.Write([]byte(res))

	if err != nil {
		fmt.Println(err, code)
	}
}

func (cfg *apiConfig) handlerMatrix(w http.ResponseWriter, _ *http.Request) {
	header := w.Header()
	header["Content-Type"] = []string{"text/html; charset=utf-8"}
	w.WriteHeader(200)
	hits := cfg.fileserverHits.Load()
	res := fmt.Sprintf(`
	<html>
		<body>
			<h1>Welcome, Chirpy Admin</h1>
			<p>Chirpy has been visited %d times!</p>
		</body>
	</html>
`, hits)
	code, err := w.Write([]byte(res))

	if err != nil {
		fmt.Println(err, code)
	}
}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	isDev := cfg.platform == devMode

	if !isDev {
		responseWithError(w, "403 Forbidden", 403)
	}

	err := cfg.db.DeleteAllUsers(r.Context())

	if err != nil {
		responseWithError(w, err.Error(), 500)
	}

	msg := struct {
		Message string `json:"message"`
	}{
		Message: "Success",
	}
	responseWithJson(w, msg, 200)

}

func (cfg *apiConfig) handlerCreateChirp(w http.ResponseWriter, r *http.Request) {

	incomingData := struct {
		Body string `json:"body"`
	}{}

	token, err := auth.GetBearerToken(r.Header)

	if err != nil {
		responseWithError(w, "Unauthorized!", 401)
	}

	userId, err := auth.ValidateJWT(token, cfg.jwtSecret)

	if err != nil {
		responseWithError(w, "Unauthorized!", 401)
	}

	jsonDecoder(w, r, &incomingData)

	if len(incomingData.Body) > 140 {
		responseWithError(w, "Chirp is too long", 400)
	}

	chirpSlice := strings.Split(incomingData.Body, " ")
	wordsToReplace := []string{"kerfuffle", "sharbert", "fornax"}
	for i, word := range chirpSlice {
		if slices.Contains(wordsToReplace, strings.ToLower(word)) {
			chirpSlice[i] = "****"
		}
	}

	cleanedBody := strings.Join(chirpSlice, " ")

	if err != nil {
		responseWithError(w, "Invalid User Id", 400)
	}

	chirp, err := cfg.db.CreateChirp(r.Context(), database.CreateChirpParams{
		Body:   cleanedBody,
		UserID: userId,
	})

	responseWithJson(w, chirp, 201)
}

func (cfg *apiConfig) handlerGetChirps(w http.ResponseWriter, r *http.Request) {

	authorId := r.URL.Query().Get("author_id")
	sortBy := r.URL.Query().Get("sort")

	userId, _ := uuid.Parse(authorId)

	chirps, err := cfg.db.GetAllChirps(r.Context())

	if err != nil {
		responseWithError(w, "Server Error", 500)
		return
	}

	resChirps := []database.Chirp{}

	if userId != uuid.Nil {
		for _, chirp := range chirps {
			if chirp.UserID == userId {
				resChirps = append(resChirps, chirp)
			}
		}
	} else {
		resChirps = chirps
	}

	if sortBy == "desc" {
		slices.SortFunc(resChirps, func(chirp1, chirp2 database.Chirp) int {
			return chirp2.CreatedAt.Compare(chirp1.CreatedAt)
		})
	}

	responseWithJson(w, resChirps, 200)
}

func (cfg *apiConfig) handlerGetChirp(w http.ResponseWriter, r *http.Request) {

	path := r.PathValue("chirpID")
	id, err := uuid.Parse(path)

	if err != nil {
		responseWithError(w, "Invalid chirp id", 400)
	}

	chirp, err := cfg.db.GetChirpById(r.Context(), id)

	if err != nil {
		if err == sql.ErrNoRows {
			responseWithError(w, "Chirp not found", 404)
		} else {
			responseWithError(w, "Server Error", 500)
		}
		return
	}
	responseWithJson(w, chirp, 200)
}

func (cfg *apiConfig) handleDeleteChirp(w http.ResponseWriter, r *http.Request) {

	path := r.PathValue("chirpID")
	id, err := uuid.Parse(path)

	if err != nil {
		responseWithError(w, "Invalid chirp id", 400)
	}

	chirp, err := cfg.db.GetChirpById(r.Context(), id)

	if err != nil {
		if err == sql.ErrNoRows {
			responseWithError(w, "Chirp not found", 404)
		} else {
			responseWithError(w, "Server Error", 500)
		}
		return
	}

	accessToken, err := auth.GetBearerToken(r.Header)

	if err != nil {
		responseWithError(w, "Unauthorized!", 401)
		return
	}

	userId, err := auth.ValidateJWT(accessToken, cfg.jwtSecret)

	if err != nil {
		responseWithError(w, "Unauthorized!", 401)
		return
	}

	if chirp.UserID != userId {
		responseWithError(w, "Unauthorized!", 403)
		return
	}

	err = cfg.db.DeletechirpById(r.Context(), chirp.ID)

	if err != nil {
		if err == sql.ErrNoRows {
			responseWithError(w, "Chirp not found", 404)
		} else {
			responseWithError(w, "Server Error", 500)
		}
		return
	}

	responseWithJson(w, struct{}{}, 204)
}

func (cfg *apiConfig) handlerCreateUser(w http.ResponseWriter, r *http.Request) {
	incomingData := struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}{}

	jsonDecoder(w, r, &incomingData)

	// fmt.Println("incoming email", incomingData.Email)

	hash, err := auth.HashPassword(incomingData.Password)

	if err != nil {
		responseWithError(w, "Server Error", 500)
	}

	user, err := cfg.db.CreateUser(context.Background(), database.CreateUserParams{
		Email:          incomingData.Email,
		HashedPassword: hash,
	})

	if err != nil {
		responseWithError(w, err.Error(), 500)
	}

	userRes := struct {
		Id          string `json:"id"`
		Email       string `json:"email"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
		IsChirpyRed bool   `json:"is_chirpy_red"`
	}{
		Id:          user.ID.String(),
		Email:       user.Email,
		CreatedAt:   user.CreatedAt.String(),
		UpdatedAt:   user.UpdatedAt.String(),
		IsChirpyRed: user.IsChirpyRed,
	}

	responseWithJson(w, userRes, 201)
}

func (cfg *apiConfig) handlerLogin(w http.ResponseWriter, r *http.Request) {
	incomingData := struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}{}

	jsonDecoder(w, r, &incomingData)

	user, err := cfg.db.FindUserByEmail(r.Context(), incomingData.Email)

	if err != nil {
		if err == sql.ErrNoRows {
			responseWithError(w, "User not found!", 404)
		} else {
			responseWithError(w, "Server error", 500)
		}
		fmt.Println(err)
		return
	}

	ok, err := auth.CheckPasswordHash(incomingData.Password, user.HashedPassword)

	if err != nil {
		responseWithError(w, "Server Error", 500)
		fmt.Println(err)
		return
	}

	if !ok {
		responseWithError(w, "Incorrect email or password", 401)
		return
	}

	expiresIn := 1 * time.Hour

	token, err := auth.MakeJWT(user.ID, cfg.jwtSecret, expiresIn)

	refToken, err := auth.MakeRefreshToken()

	if err != nil {
		responseWithError(w, "server error", 500)
		fmt.Println(err)
		return
	}

	dbRefToken, err := cfg.db.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		Token:     refToken,
		ExpiresAt: time.Now().UTC().Add(time.Hour * 24 * 60),
		UserID:    user.ID,
	})

	if err != nil {
		responseWithError(w, "Server error", 500)
		fmt.Println(err)
		return
	}

	userRes := struct {
		Id           string `json:"id"`
		Email        string `json:"email"`
		CreatedAt    string `json:"created_at"`
		UpdatedAt    string `json:"updated_at"`
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
		IsChirpyRed  bool   `json:"is_chirpy_red"`
	}{
		Id:           user.ID.String(),
		Email:        user.Email,
		CreatedAt:    user.CreatedAt.String(),
		UpdatedAt:    user.UpdatedAt.String(),
		IsChirpyRed:  user.IsChirpyRed,
		Token:        token,
		RefreshToken: dbRefToken.Token,
	}

	responseWithJson(w, userRes, 200)
}

func (cfg *apiConfig) handlerRefresh(w http.ResponseWriter, r *http.Request) {

	refreshToken, err := auth.GetBearerToken(r.Header)

	if err != nil {
		responseWithError(w, "Unauthorized!", 401)
	}

	dbRefreshToken, err := cfg.db.GetDBRefreshTokenByRefreshToken(r.Context(), refreshToken)

	if err != nil {
		if err == sql.ErrNoRows {
			responseWithError(w, "Unauthorized", 401)
		}
		responseWithError(w, "Server Error", 500)
	}

	if dbRefreshToken.RevokedAt.Valid {
		responseWithError(w, "Unauthorized", 401)
		return
	}

	expiresIn := 1 * time.Hour

	token, err := auth.MakeJWT(dbRefreshToken.UserID, cfg.jwtSecret, expiresIn)

	if err != nil {
		responseWithError(w, "server error", 500)
	}

	userRes := struct {
		Token string `json:"token"`
	}{
		Token: token,
	}

	responseWithJson(w, userRes, 200)
}

func (cfg *apiConfig) handlerRevoke(w http.ResponseWriter, r *http.Request) {

	refreshToken, err := auth.GetBearerToken(r.Header)

	if err != nil {
		responseWithError(w, "Unauthorized!", 401)
	}

	dbRefreshToken, err := cfg.db.GetDBRefreshTokenByRefreshToken(r.Context(), refreshToken)

	if err != nil {
		if err == sql.ErrNoRows {
			responseWithError(w, "Unauthorized", 401)
		}
		responseWithError(w, "Server Error", 500)
	}

	if dbRefreshToken.RevokedAt.Valid {
		responseWithError(w, "Unauthorized", 401)
		return
	}

	err = cfg.db.RevokeRefreshToken(r.Context(), refreshToken)

	if err != nil {
		responseWithError(w, "Server Error", 500)
	}

	responseWithJson(w, struct{}{}, 204)
}

func (cfg *apiConfig) handlerUpdateUser(w http.ResponseWriter, r *http.Request) {
	incomingData := struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}{}

	jsonDecoder(w, r, &incomingData)

	accessToken, err := auth.GetBearerToken(r.Header)

	if err != nil {
		responseWithError(w, "Unauthorized!", 401)
		return
	}

	userId, err := auth.ValidateJWT(accessToken, cfg.jwtSecret)

	if err != nil {
		responseWithError(w, "Unauthorized!", 401)
		return
	}

	user, err := cfg.db.GetUserById(r.Context(), userId)

	if err != nil {
		if err == sql.ErrNoRows {
			responseWithError(w, "Unauthorized", 401)
		} else {
			responseWithError(w, "Server error", 500)
		}
		return
	}

	hash, err := auth.HashPassword(incomingData.Password)

	if err != nil {
		responseWithError(w, "Server Error", 500)
		return
	}

	updatedUser, err := cfg.db.UpdateUserById(r.Context(), database.UpdateUserByIdParams{
		HashedPassword: hash,
		Email:          incomingData.Email,
		ID:             user.ID,
	})

	if err != nil {
		responseWithError(w, err.Error(), 500)
		return
	}

	userRes := struct {
		Id          string `json:"id"`
		Email       string `json:"email"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
		IsChirpyRed bool   `json:"is_chirpy_red"`
	}{
		Id:          updatedUser.ID.String(),
		Email:       updatedUser.Email,
		CreatedAt:   updatedUser.CreatedAt.String(),
		UpdatedAt:   updatedUser.UpdatedAt.String(),
		IsChirpyRed: updatedUser.IsChirpyRed,
	}

	responseWithJson(w, userRes, 200)
}

func (cfg *apiConfig) handlerUpgradeUser(w http.ResponseWriter, r *http.Request) {

	reqKey, err := auth.GetApiKey(r.Header)

	if err != nil || reqKey != cfg.polkaApiKey {
		responseWithError(w, "Forbidden", 401)
	}

	incomingData := struct {
		Even string `json:"event"`
		Data struct {
			UserId string `json:"user_id"`
		} `json:"data"`
	}{}

	jsonDecoder(w, r, &incomingData)

	if incomingData.Even != "user.upgraded" {
		responseWithJson(w, struct{}{}, 204)
		return
	}

	userId, err := uuid.Parse(incomingData.Data.UserId)

	if err != nil {
		responseWithError(w, "User Not found", 404)
		return
	}

	user, err := cfg.db.GetUserById(r.Context(), userId)

	if err != nil {
		if err == sql.ErrNoRows {
			responseWithError(w, "User Not found", 404)
		} else {
			responseWithError(w, "Server error", 500)
		}
		return
	}

	err = cfg.db.UpgradeUserToRed(r.Context(), user.ID)

	if err != nil {
		if err == sql.ErrNoRows {
			responseWithError(w, "User Not found", 404)
		} else {
			responseWithError(w, "Server error", 500)
		}
		return
	}

	responseWithJson(w, struct{}{}, 204)
}

func main() {

	godotenv.Load()

	dbURL := os.Getenv("DB_URL")

	db, err := sql.Open("postgres", dbURL)

	if err != nil {
		fmt.Printf("Error connecting db: %v", err)
	}

	dbQueries := database.New(db)

	config := apiConfig{
		fileserverHits: atomic.Int32{},
		db:             dbQueries,
		platform:       os.Getenv("PLATFORM"),
		jwtSecret:      os.Getenv("JWT_SECRET"),
		polkaApiKey:    os.Getenv("POLKA_KEY"),
	}

	mux := http.ServeMux{}

	mux.HandleFunc("GET /api/healthz", handlerHealthz)

	mux.HandleFunc("GET /admin/metrics", config.handlerMatrix)

	mux.HandleFunc("POST /admin/reset", config.handlerReset)

	mux.HandleFunc("POST /api/chirps", config.handlerCreateChirp)

	mux.HandleFunc("GET /api/chirps", config.handlerGetChirps)

	mux.HandleFunc("GET /api/chirps/{chirpID}", config.handlerGetChirp)

	mux.HandleFunc("DELETE /api/chirps/{chirpID}", config.handleDeleteChirp)

	mux.HandleFunc("POST /api/users", config.handlerCreateUser)

	mux.HandleFunc("PUT /api/users", config.handlerUpdateUser)

	mux.HandleFunc("POST /api/login", config.handlerLogin)

	mux.HandleFunc("POST /api/refresh", config.handlerRefresh)

	mux.HandleFunc("POST /api/revoke", config.handlerRevoke)

	mux.HandleFunc("POST /api/polka/webhooks", config.handlerUpgradeUser)

	fileServer := http.FileServer(http.Dir("."))
	mux.Handle("/app/", http.StripPrefix("/app", config.middlewareMetricsInc(fileServer)))

	server := http.Server{
		Handler: &mux,
		Addr:    ":8080",
	}

	err = server.ListenAndServe()

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
