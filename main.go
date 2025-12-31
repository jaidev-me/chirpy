package main

import (
	"fmt"
	"net/http"
	"os"
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
	header["Content-Type"] = []string{"text/plain; charset=utf-8"}
	w.WriteHeader(200)
	hits := cgf.fileserverHits.Load()
	res := fmt.Sprintf("Hits: %d", hits)
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

func main() {

	config := apiConfig{
		fileserverHits: atomic.Int32{},
	}

	mux := http.ServeMux{}

	mux.HandleFunc("GET /healthz", handlerHealthz)

	mux.HandleFunc("GET /metrics", config.handlerMatrix)

	mux.HandleFunc("POST /reset", config.handlerReset)

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
