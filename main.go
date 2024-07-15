package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type apiConfig struct {
	fileserverHits int
}

func (cft *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cft.fileserverHits++
		next.ServeHTTP(w, r)
	})
}

func healthz(resp http.ResponseWriter, req *http.Request) {
	resp.Header().Set("Content-Type", "text/plain; charset=utf-8")
	resp.WriteHeader(http.StatusOK)
	resp.Write([]byte(http.StatusText(http.StatusOK)))
}

func (cft *apiConfig) getMetrics(resp http.ResponseWriter, req *http.Request) {
	resp.Header().Set("Content-Type", "text/html")
	resp.WriteHeader(http.StatusOK)
	resp.Write([]byte(fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>", cft.fileserverHits)))
}

func (cft *apiConfig) resetMetrics(resp http.ResponseWriter, req *http.Request) {
	cft.fileserverHits = 0
	resp.WriteHeader(http.StatusOK)
	resp.Write([]byte("Hits reset to 0"))
}

func validateChirp(resp http.ResponseWriter, req *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}

	decoder := json.NewDecoder(req.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		resp.WriteHeader(500)
		return
	}
	fmt.Println(params)

}

func main() {
	const filepathRoot = "."
	const port = "8080"

	config := apiConfig{fileserverHits: 0}

	mux := http.NewServeMux()
	fsHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	mux.Handle("/app/*", config.middlewareMetricsInc(fsHandler))
	mux.HandleFunc("GET /admin/metrics", config.getMetrics)
	mux.HandleFunc("GET /api/reset", config.resetMetrics)
	mux.HandleFunc("GET /api/healthz", healthz)
	mux.HandleFunc("POST /api/validate_chirp", validateChirp)

	server := &http.Server{
		Handler: mux,
		Addr:    ":" + port,
	}

	log.Printf("Serving files from %s on port: %s\n", filepathRoot, port)
	server.ListenAndServe()
}
