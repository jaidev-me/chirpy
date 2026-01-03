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

	"github.com/google/uuid"
	"github.com/jaidev-me/chirpy/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
}

const devMode = "dev"

// helpers

func responseWithError(w http.ResponseWriter, err string, statusCode int) {
	type resError struct {
		Error string `json:"error"`
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorBody, marshalError := json.Marshal(resError{Error: err})
	if marshalError != nil {
		fmt.Printf("Error while parsing json: %v", marshalError)
		return
	}
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
		Body   string `json:"body"`
		UserId string `json:"user_id"`
	}{}

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
	userId, err := uuid.Parse(incomingData.UserId)

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

	chirps, err := cfg.db.GetAllChirps(r.Context())

	if err != nil {
		responseWithError(w, "Server Error", 500)
	}

	responseWithJson(w, chirps, 200)
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

func (cfg *apiConfig) handlerCreateUser(w http.ResponseWriter, r *http.Request) {
	incomingData := struct {
		Email string `json:"email"`
	}{}

	jsonDecoder(w, r, &incomingData)

	// fmt.Println("incoming email", incomingData.Email)

	user, err := cfg.db.CreateUser(context.Background(), incomingData.Email)

	if err != nil {
		responseWithError(w, err.Error(), 500)
	}

	userRes := struct {
		Id        string `json:"id"`
		Email     string `json:"email"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}{
		Id:        user.ID.String(),
		Email:     user.Email,
		CreatedAt: user.CreatedAt.String(),
		UpdatedAt: user.UpdatedAt.String(),
	}

	responseWithJson(w, userRes, 201)
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
	}

	mux := http.ServeMux{}

	mux.HandleFunc("GET /api/healthz", handlerHealthz)

	mux.HandleFunc("GET /admin/metrics", config.handlerMatrix)

	mux.HandleFunc("POST /admin/reset", config.handlerReset)

	mux.HandleFunc("POST /api/chirps", config.handlerCreateChirp)

	mux.HandleFunc("GET /api/chirps", config.handlerGetChirps)

	mux.HandleFunc("GET /api/chirps/{chirpID}", config.handlerGetChirp)

	mux.HandleFunc("POST /api/users", config.handlerCreateUser)

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
