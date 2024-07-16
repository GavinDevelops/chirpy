package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/GavinDevelops/chirpy/database"
)

type apiConfig struct {
	fileserverHits int
}

type dbWrapper struct {
	db *database.DB
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

func (dbw dbWrapper) createUser(w http.ResponseWriter, req *http.Request) {
	type parameters struct {
		Email string `json:"email"`
	}
	decoder := json.NewDecoder(req.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't decode parameters")
		return
	}
	user, createErr := dbw.db.CreateUser(params.Email)
	if createErr != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating user")
		return
	}
	respondWithJson(w, http.StatusCreated, user)
}

func (dbw dbWrapper) getChirp(w http.ResponseWriter, req *http.Request) {
	id, parseErr := strconv.Atoi(req.PathValue("chirpid"))
	if parseErr != nil {
		respondWithError(w, http.StatusBadRequest, "Error parsing path param")
		return
	}
	chirp, err := dbw.db.GetChirp(id)
	if err != nil {
		respondWithError(w, http.StatusNotFound, err.Error())
		return
	}
	respondWithJson(w, http.StatusOK, chirp)

}

func (dbw dbWrapper) getChirps(w http.ResponseWriter, req *http.Request) {
	chirps, err := dbw.db.GetChirps()
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get chirps")
		return
	}
	respondWithJson(w, http.StatusOK, chirps)
}

func (dbw dbWrapper) validateChirp(resp http.ResponseWriter, req *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}
	type reqValid struct {
		CleanedBody string `json:"cleaned_body"`
	}

	decoder := json.NewDecoder(req.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(resp, http.StatusInternalServerError, "Couldn't decode parameters")
		return
	}
	const maxChirpLen = 140
	if len(params.Body) >= maxChirpLen {
		respondWithError(resp, http.StatusBadRequest, "Chirp is too long")
		return
	}
	chirp, createErr := dbw.db.CreateChirp(params.Body)
	if createErr != nil {
		respondWithError(resp, http.StatusBadRequest, "Couldn't create chirp")
		return
	}
	respondWithJson(resp, http.StatusCreated, chirp)
}

func cleanMessage(msg string) string {
	badWords := map[string]bool{"kerfuffle": true, "sharbert": true, "fornax": true}
	replacement := "****"
	splitMsg := strings.Split(msg, " ")
	cleanedMessages := make([]string, 0)
	for _, msg := range splitMsg {
		if badWords[strings.ToLower(msg)] {
			cleanedMessages = append(cleanedMessages, replacement)
			continue
		}
		cleanedMessages = append(cleanedMessages, msg)
	}
	joined := strings.Join(cleanedMessages, " ")
	return joined
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	if code > 499 {
		log.Printf("Responding with 5XX error: %s\n", msg)
	}
	type errorResponse struct {
		Error string `json:"error"`
	}
	respondWithJson(w, code, errorResponse{Error: msg})
}

func respondWithJson(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(code)
	w.Write(dat)
}

func main() {
	const filepathRoot = "."
	const port = "8080"

	config := apiConfig{fileserverHits: 0}
	db, err := database.NewDB("./database.json")
	if err != nil {
		log.Fatalf("Error starting database: %s", err)
	}
	dbw := dbWrapper{db: db}

	mux := http.NewServeMux()
	fsHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	mux.Handle("/app/*", config.middlewareMetricsInc(fsHandler))
	mux.HandleFunc("GET /admin/metrics", config.getMetrics)
	mux.HandleFunc("GET /api/reset", config.resetMetrics)
	mux.HandleFunc("GET /api/healthz", healthz)
	mux.HandleFunc("POST /api/chirps", dbw.validateChirp)
	mux.HandleFunc("GET /api/chirps", dbw.getChirps)
	mux.HandleFunc("GET /api/chirps/{chirpid}", dbw.getChirp)
	mux.HandleFunc("POST /api/users", dbw.createUser)

	server := &http.Server{
		Handler: mux,
		Addr:    ":" + port,
	}

	log.Printf("Serving files from %s on port: %s\n", filepathRoot, port)
	server.ListenAndServe()
}
