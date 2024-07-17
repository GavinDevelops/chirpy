package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/GavinDevelops/chirpy/database"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
)

type apiConfig struct {
	fileserverHits int
	jwtSecret      string
	db             *database.DB
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

func (cft *apiConfig) createUser(w http.ResponseWriter, req *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(req.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't decode parameters")
		return
	}
	user, createErr := cft.db.CreateUser(params.Email, params.Password)
	if createErr != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating user")
		return
	}
	respondWithJson(w, http.StatusCreated, user)
}

func (cft *apiConfig) getUser(w http.ResponseWriter, req *http.Request) {
	type parameters struct {
		Email            string `json:"email"`
		Password         string `json:"password"`
		ExpiresInSeconds int    `json:"expires_in_seconds"`
	}
	decoder := json.NewDecoder(req.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't decode parameters")
		return
	}
	user, verifyErr := cft.db.VerifyUser(params.Email, params.Password, cft.jwtSecret, params.ExpiresInSeconds)
	if verifyErr != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid password")
		return
	}
	respondWithJson(w, http.StatusOK, user)
}

func (cft *apiConfig) updateUser(w http.ResponseWriter, req *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	token := req.Header.Get("Authorization")
	token = strings.TrimPrefix(token, "Bearer ")
	jwtToken, err := jwt.ParseWithClaims(
		token,
		&jwt.RegisteredClaims{},
		func(token *jwt.Token) (
			interface{},
			error,
		) {
			return []byte(cft.jwtSecret), nil
		},
	)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	id, idErr := jwtToken.Claims.GetSubject()
	if idErr != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting id from subjet")
		return
	}
	decoder := json.NewDecoder(req.Body)
	params := parameters{}
	decodeErr := decoder.Decode(&params)
	if decodeErr != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't decode parameters")
		return
	}
	user, updateUserErr := cft.db.UpdateUser(id, params.Email, params.Password)
	if updateUserErr != nil {
		respondWithError(w, http.StatusInternalServerError, updateUserErr.Error())
		return
	}
	respondWithJson(w, http.StatusOK, user)
}

func (cft *apiConfig) getChirp(w http.ResponseWriter, req *http.Request) {
	id, parseErr := strconv.Atoi(req.PathValue("chirpid"))
	if parseErr != nil {
		respondWithError(w, http.StatusBadRequest, "Error parsing path param")
		return
	}
	chirp, err := cft.db.GetChirp(id)
	if err != nil {
		respondWithError(w, http.StatusNotFound, err.Error())
		return
	}
	respondWithJson(w, http.StatusOK, chirp)

}

func (cft *apiConfig) getChirps(w http.ResponseWriter, req *http.Request) {
	chirps, err := cft.db.GetChirps()
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get chirps")
		return
	}
	respondWithJson(w, http.StatusOK, chirps)
}

func (cft *apiConfig) validateChirp(resp http.ResponseWriter, req *http.Request) {
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
	chirp, createErr := cft.db.CreateChirp(params.Body)
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
	godotenv.Load()
	jwtSecret := os.Getenv("JWT_SECRET")
	const filepathRoot = "."
	const port = "8080"

	db, err := database.NewDB("./database.json")
	if err != nil {
		log.Fatalf("Error starting database: %s", err)
	}
	config := apiConfig{fileserverHits: 0, jwtSecret: jwtSecret, db: db}

	mux := http.NewServeMux()
	fsHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	mux.Handle("/app/*", config.middlewareMetricsInc(fsHandler))
	mux.HandleFunc("GET /admin/metrics", config.getMetrics)
	mux.HandleFunc("GET /api/reset", config.resetMetrics)
	mux.HandleFunc("GET /api/healthz", healthz)
	mux.HandleFunc("POST /api/chirps", config.validateChirp)
	mux.HandleFunc("GET /api/chirps", config.getChirps)
	mux.HandleFunc("GET /api/chirps/{chirpid}", config.getChirp)
	mux.HandleFunc("POST /api/users", config.createUser)
	mux.HandleFunc("POST /api/login", config.getUser)
	mux.HandleFunc("PUT /api/users", config.updateUser)

	server := &http.Server{
		Handler: mux,
		Addr:    ":" + port,
	}

	log.Printf("Serving files from %s on port: %s\n", filepathRoot, port)
	server.ListenAndServe()
}
