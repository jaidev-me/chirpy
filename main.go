package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync/atomic"
)

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

func (cgf *apiConfig) handlerMatrix(w http.ResponseWriter, _ *http.Request) {
	header := w.Header()
	header["Content-Type"] = []string{"text/html; charset=utf-8"}
	w.WriteHeader(200)
	hits := cgf.fileserverHits.Load()
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

func (cgf *apiConfig) handlerReset(w http.ResponseWriter, _ *http.Request) {
	header := w.Header()
	header["Content-Type"] = []string{"text/plain; charset=utf-8"}
	w.WriteHeader(200)
	cgf.fileserverHits.Store(int32(0))
	code, err := w.Write([]byte("Done"))

	if err != nil {
		fmt.Println(err, code)
	}
}

func handlerChirpValidation(w http.ResponseWriter, r *http.Request) {

	type resError struct {
		Error string `json:"error"`
	}

	incomingData := struct {
		Body string `json:"body"`
	}{}

	w.Header().Set("Content-Type", "application/json")

	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	if err := decoder.Decode(&incomingData); err != nil {
		errorBody, _ := json.Marshal(resError{Error: "Something went wrong!"})
		w.WriteHeader(500)
		w.Write(errorBody)
	}

	if len(incomingData.Body) > 140 {
		errorBody, _ := json.Marshal(resError{Error: "Chirp is too long"})
		w.WriteHeader(400)
		w.Write(errorBody)
	}

	w.WriteHeader(200)

	chirpSlice := strings.Split(incomingData.Body, " ")
	wordsToReplace := []string{"kerfuffle", "sharbert", "fornax"}
	for i, word := range chirpSlice {
		if slices.Contains(wordsToReplace, strings.ToLower(word)) {
			chirpSlice[i] = "****"
		}
	}

	body, err := json.Marshal(struct {
		CleanedBody string `json:"cleaned_body"`
	}{CleanedBody: strings.Join(chirpSlice, " ")})

	if err != nil {
		errorBody, _ := json.Marshal(resError{Error: "Something went wrong!"})
		w.WriteHeader(500)
		w.Write(errorBody)
	}

	w.Write([]byte(body))
}

func main() {

	config := apiConfig{
		fileserverHits: atomic.Int32{},
	}

	mux := http.ServeMux{}

	mux.HandleFunc("GET /api/healthz", handlerHealthz)

	mux.HandleFunc("GET /admin/metrics", config.handlerMatrix)

	mux.HandleFunc("POST /admin/reset", config.handlerReset)

	mux.HandleFunc("POST /api/validate_chirp", handlerChirpValidation)

	fileServer := http.FileServer(http.Dir("."))
	mux.Handle("/app/", http.StripPrefix("/app", config.middlewareMetricsInc(fileServer)))

	server := http.Server{
		Handler: &mux,
		Addr:    ":8080",
	}

	err := server.ListenAndServe()

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
