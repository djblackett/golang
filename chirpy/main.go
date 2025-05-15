package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/djblackett/chirpy/internal/auth"
	"github.com/djblackett/chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	platform       string
	dbQueries      *database.Queries
	jwtSecret      string
	polkaKey       string
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, cfg.fileserverHits.Load())))
}

func (cfg *apiConfig) reset(w http.ResponseWriter, r *http.Request) {
	if cfg.platform == "dev" {
		err := cfg.dbQueries.DeleteUsers(r.Context())
		if err != nil {
			log.Printf("Error deleting users: %s", err)
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusForbidden)
	}
}

type ChirpParameters struct {
	UserID uuid.UUID `json:"user_id"`
	Body   string    `json:"body"`
}

// main function to start the HTTP server
func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")
	jwtSecret := os.Getenv("JWT_SECRET")
	polkaKey := os.Getenv("POLKA_KEY")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Error opening database: %s", err)
	}
	dbQueries := database.New(db)
	defer db.Close()
	apiCfg := apiConfig{
		fileserverHits: atomic.Int32{},
		dbQueries:      dbQueries,
		platform:       platform,
		jwtSecret:      jwtSecret,
		polkaKey:       polkaKey,
	}

	fmt.Println("Starting server on :8080")
	serveMux := http.NewServeMux()
	serveMux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	serveMux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		bytes := []byte("OK")
		w.Write(bytes)
	})

	serveMux.HandleFunc("POST /api/chirps", func(w http.ResponseWriter, r *http.Request) {

		decoder := json.NewDecoder(r.Body)
		params := ChirpParameters{}
		err := decoder.Decode(&params)
		if err != nil {
			// an error will be thrown if the JSON is invalid or has the wrong types
			// any missing fields will simply have their values in the struct set to their zero value
			log.Printf("POST /api/chirps - Error decoding parameters: %s", err)
			w.WriteHeader(500)
			return
		}

		token, err := auth.GetBearerToken(r.Header)
		if err != nil {
			log.Printf("POST /api/chirps - Error getting Bearer token: %s", err)
			w.WriteHeader(401)
			return
		}

		userID, err := auth.ValidateJWT(token, apiCfg.jwtSecret)

		if err != nil {
			log.Printf("POST /api/chirps - Error validating JWT: %s - %v", err, token)
			w.WriteHeader(401)
			return
		}

		isBodyValid := len(params.Body) <= 140

		words := replaceBadWords(params)

		if isBodyValid {
			newChirp := ChirpParameters{
				Body: strings.Join(words, " "),
			}

			chirp, err := apiCfg.dbQueries.CreateChirp(r.Context(), database.CreateChirpParams{
				UserID: userID,
				Body:   newChirp.Body,
			})
			if err != nil {
				log.Printf("POST /api/chirps - Error creating chirp: %s", err)
				w.WriteHeader(500)
				return
			}

			returnedChirp := Chirp{
				ID:        chirp.ID,
				UserID:    chirp.UserID.String(),
				CreatedAt: chirp.CreatedAt.Time,
				UpdatedAt: chirp.UpdatedAt.Time,
				Body:      chirp.Body,
			}

			bytes, err := json.Marshal(returnedChirp)
			if err != nil {
				log.Printf("POST /api/chirps - Error marshalling JSON: %s", err)
				w.WriteHeader(500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write(bytes)

		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	})

	serveMux.HandleFunc("GET /api/chirps", func(w http.ResponseWriter, r *http.Request) {
		var chirps []database.Chirp
		authorID := r.URL.Query().Get("author_id")
		sortBy := r.URL.Query().Get("sort")

		if authorID != "" {
			authorUUID, err := uuid.Parse(authorID)
			if err != nil {
				log.Printf("Error parsing authorID as UUID: %s", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			chirps, err = apiCfg.dbQueries.GetChirpsByUserID(r.Context(), authorUUID)
			if err != nil {
				log.Printf("Error getting chirps by user ID: %s", err)
				w.WriteHeader(500)
				return
			}
		} else {
			chirps, err = apiCfg.dbQueries.GetChirps(r.Context())
			if err != nil {
				log.Printf("Error getting chirps: %s", err)
				w.WriteHeader(500)
				return
			}
		}

		var chirpList []Chirp
		for _, chirp := range chirps {
			chirpList = append(chirpList, Chirp{
				ID:        chirp.ID,
				UserID:    chirp.UserID.String(),
				CreatedAt: chirp.CreatedAt.Time,
				UpdatedAt: chirp.UpdatedAt.Time,
				Body:      chirp.Body,
			})
		}

		if sortBy == "desc" {
			sort.Slice(chirpList, func(i, j int) bool {
				return chirpList[i].CreatedAt.After(chirpList[j].CreatedAt)
			})
		}

		bytes, err := json.Marshal(chirpList)
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(bytes)
	})

	serveMux.HandleFunc("GET /api/chirps/{chirpID}", func(w http.ResponseWriter, r *http.Request) {
		chirpID := r.PathValue("chirpID")
		if chirpID == "" {
			log.Printf("Error: chirpID is empty")
			w.WriteHeader(400)
			return
		}

		chirpUUID, err := uuid.Parse(chirpID)
		if err != nil {
			log.Printf("Error parsing chirpID as UUID: %s", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		chirp, err := apiCfg.dbQueries.GetChirp(r.Context(), chirpUUID)
		if err != nil {
			log.Printf("Error getting chirp: %s", err)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		returnedChirp := Chirp{
			ID:        chirp.ID,
			UserID:    chirp.UserID.String(),
			CreatedAt: chirp.CreatedAt.Time,
			UpdatedAt: chirp.UpdatedAt.Time,
			Body:      chirp.Body,
		}
		bytes, err := json.Marshal(returnedChirp)
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(bytes)
	})
	serveMux.HandleFunc("GET /admin/metrics", apiCfg.handleMetrics)
	serveMux.HandleFunc("POST /admin/reset", apiCfg.reset)
	serveMux.HandleFunc("POST /api/users", func(w http.ResponseWriter, r *http.Request) {

		type parameters struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		decoder := json.NewDecoder(r.Body)
		params := parameters{}
		err := decoder.Decode(&params)
		if err != nil {
			log.Printf("Error decoding request body: %s", err)
			w.WriteHeader(500)
			return
		}
		if params.Email == "" {
			log.Printf("Error: email is empty")
			w.WriteHeader(400)
			return
		}

		if params.Password == "" {
			log.Printf("Error: password is empty")
			w.WriteHeader(400)
			return
		}

		params.Password, err = auth.HashPassword(params.Password)
		if err != nil {
			log.Printf("Error hashing password: %s", err)
			w.WriteHeader(500)
			return
		}

		user, err := apiCfg.dbQueries.CreateUser(r.Context(), database.CreateUserParams{Email: params.Email, HashedPassword: params.Password})
		if err != nil {
			log.Printf("Error creating user: %s", err)
			w.WriteHeader(500)
			return
		}

		returnedUser := User{
			ID:          user.ID,
			CreatedAt:   user.CreatedAt.Time,
			UpdatedAt:   user.UpdatedAt.Time,
			Email:       user.Email,
			IsChirpyRed: user.IsChirpyRed.Bool,
		}

		bytes, err := json.Marshal(returnedUser)
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write(bytes)
	})

	serveMux.HandleFunc("PUT /api/users", func(w http.ResponseWriter, r *http.Request) {
		type parameters struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		token, err := auth.GetBearerToken(r.Header)

		if err != nil {
			log.Printf("Error getting Bearer token: %s", err)
			w.WriteHeader(401)
			return
		}

		userID, err := auth.ValidateJWT(token, apiCfg.jwtSecret)

		if err != nil {
			log.Printf("Error validating JWT: %s", err)
			w.WriteHeader(401)
			return
		}

		decoder := json.NewDecoder(r.Body)
		params := parameters{}
		err = decoder.Decode(&params)
		if err != nil {
			log.Printf("Error decoding request body: %s", err)
			w.WriteHeader(500)
			return
		}

		if params.Email == "" {
			log.Printf("Error: email is empty")
			w.WriteHeader(400)
			return
		}

		if params.Password == "" {
			log.Printf("Error: password is empty")
			w.WriteHeader(400)
			return
		}

		params.Password, err = auth.HashPassword(params.Password)
		if err != nil {
			log.Printf("Error hashing password: %s", err)
			w.WriteHeader(500)
			return
		}

		user, err := apiCfg.dbQueries.UpdateUser(r.Context(), database.UpdateUserParams{
			Email:          params.Email,
			HashedPassword: params.Password,
			ID:             userID,
		})

		if err != nil {
			log.Printf("Error updating user: %s", err)
			w.WriteHeader(500)
			return
		}

		returnedUser := User{
			ID:          user.ID,
			CreatedAt:   user.CreatedAt.Time,
			UpdatedAt:   user.UpdatedAt.Time,
			Email:       user.Email,
			IsChirpyRed: user.IsChirpyRed.Bool,
		}

		bytes, err := json.Marshal(returnedUser)

		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(bytes)
	})

	serveMux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		type parameters struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		decoder := json.NewDecoder(r.Body)
		params := parameters{}
		err := decoder.Decode(&params)
		if err != nil {
			log.Printf("Error decoding request body: %s", err)
			w.WriteHeader(500)
			return
		}
		if params.Email == "" {
			log.Printf("Error: email is empty")
			w.WriteHeader(400)
			return
		}
		if params.Password == "" {
			log.Printf("Error: password is empty")
			w.WriteHeader(400)
			return
		}

		user, err := apiCfg.dbQueries.GetUserByEmail(r.Context(), params.Email)
		if err != nil {
			log.Printf("Error getting user by email: %s", err)
			w.WriteHeader(500)
			return
		}

		err = auth.CheckPasswordHash(user.HashedPassword, params.Password)
		if err != nil {
			log.Printf("Error checking password hash: %s", err)
			w.WriteHeader(401)
			return
		}

		duration := time.Hour

		token, err := auth.MakeJWT(user.ID, apiCfg.jwtSecret, duration)
		if err != nil {
			log.Printf("Error creating JWT: %s", err)
			w.WriteHeader(500)
			return
		}

		refresh_token, err := auth.MakeRefreshToken()
		if err != nil {
			log.Printf("Error creating refresh token: %s", err)
			w.WriteHeader(500)
			return
		}

		err = apiCfg.dbQueries.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
			UserID: user.ID,
			Token:  refresh_token,
		})

		if err != nil {
			log.Printf("Error creating refresh token in database: %s", err)
			w.WriteHeader(500)
			return
		}

		type tokenResponse struct {
			Token        string    `json:"token"`
			ID           uuid.UUID `json:"id"`
			CreatedAt    time.Time `json:"created_at"`
			UpdatedAt    time.Time `json:"updated_at"`
			Email        string    `json:"email"`
			IsChirpyRed  bool      `json:"is_chirpy_red"`
			RefreshToken string    `json:"refresh_token"`
		}
		response := tokenResponse{
			Token:        token,
			ID:           user.ID,
			CreatedAt:    user.CreatedAt.Time,
			UpdatedAt:    user.UpdatedAt.Time,
			Email:        user.Email,
			IsChirpyRed:  user.IsChirpyRed.Bool,
			RefreshToken: refresh_token,
		}

		bytes, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(bytes)
	})

	serveMux.HandleFunc("POST /api/refresh", func(w http.ResponseWriter, r *http.Request) {

		refreshToken, err := auth.GetBearerToken(r.Header)
		if err != nil {
			log.Printf("Error getting Bearer token: %s", err)
			w.WriteHeader(401)
			return
		}
		userID, err := apiCfg.dbQueries.GetUserByRefreshToken(r.Context(), refreshToken)
		if err != nil {
			log.Printf("POST /api/refresh - Error getting user by refresh token: %s - %v", err, userID)
			w.WriteHeader(401)
			return
		}

		accessToken, err := auth.MakeJWT(userID, apiCfg.jwtSecret, time.Hour)
		if err != nil {
			log.Printf("Error creating JWT: %s", err)
			w.WriteHeader(500)
			return
		}

		type tokenResponse struct {
			Token string `json:"token"`
		}

		response := tokenResponse{
			Token: accessToken,
		}
		bytes, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(bytes)
	})

	serveMux.HandleFunc("POST /api/revoke", func(w http.ResponseWriter, r *http.Request) {

		refreshToken, err := auth.GetBearerToken(r.Header)
		if err != nil {
			log.Printf("Error getting Bearer token: %s", err)
			w.WriteHeader(401)
			return
		}

		_, err = apiCfg.dbQueries.RevokeRefreshToken(r.Context(), refreshToken)
		if err != nil {
			log.Printf("Error revoking refresh token: %s", err)
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	serveMux.HandleFunc("DELETE /api/chirps/{chirpID}", func(w http.ResponseWriter, r *http.Request) {
		chirpID := r.PathValue("chirpID")
		if chirpID == "" {
			log.Printf("Error: chirpID is empty")
			w.WriteHeader(400)
			return
		}

		chirpUUID, err := uuid.Parse(chirpID)
		if err != nil {
			log.Printf("Error parsing chirpID as UUID: %s", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		chirp, err := apiCfg.dbQueries.GetChirp(r.Context(), chirpUUID)
		if err != nil {
			log.Printf("Error: chirp not found")
			w.WriteHeader(404)
			return
		}

		token, err := auth.GetBearerToken(r.Header)
		if err != nil {
			log.Printf("Error getting Bearer token: %s", err)
			w.WriteHeader(401)
			return
		}
		userID, err := auth.ValidateJWT(token, apiCfg.jwtSecret)
		if err != nil {
			log.Printf("Error validating JWT: %s", err)
			w.WriteHeader(401)
			return
		}
		if userID != chirp.UserID {
			log.Printf("Error: chirp does not belong to user")
			w.WriteHeader(403)
			return
		}

		err = apiCfg.dbQueries.DeleteChirp(r.Context(), chirpUUID)
		if err != nil {
			log.Printf("Error deleting chirp: %s", err)
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	serveMux.HandleFunc("POST /api/polka/webhooks", func(w http.ResponseWriter, r *http.Request) {
		type event struct {
			Event string `json:"event"`
			Data  struct {
				UserID string `json:"user_id"`
			} `json:"data"`
		}

		apiKey, err := auth.GetAPIKey(r.Header)
		if err != nil {
			log.Printf("Error getting API key: %s - apiKey: %v", err, apiKey)
			w.WriteHeader(401)
			return
		}
		if strings.TrimSpace(apiKey) != strings.TrimSpace(apiCfg.polkaKey) {
			log.Printf("Error: API key does not match - apiKey: %v", apiKey)
			log.Printf("Error: API key does not match - polkaKey: %v", apiCfg.polkaKey)
			w.WriteHeader(401)
			return
		}

		decoder := json.NewDecoder(r.Body)
		params := event{}
		err = decoder.Decode(&params)
		if err != nil {
			log.Printf("Error decoding request body: %s", err)
			w.WriteHeader(500)
			return
		}

		if params.Event != "user.upgraded" {
			w.WriteHeader(204)
			return
		}

		userID, err := uuid.Parse(params.Data.UserID)
		if err != nil {
			log.Printf("Error parsing userID as UUID: %s", err)
			w.WriteHeader(400)
			return
		}

		user, err := apiCfg.dbQueries.GetUserByID(r.Context(), userID)
		if err != nil {
			log.Printf("User not found: %s", err)
			w.WriteHeader(404)
			return
		}

		user, err = apiCfg.dbQueries.UpgradeUserToRed(r.Context(), user.ID)
		if err == nil {
			log.Printf("User upgraded to red: %s", user.ID)
			w.WriteHeader(204)
			return
		}
	})
	server := http.Server{
		Addr:    ":8080",
		Handler: serveMux,
	}

	server.ListenAndServe()
}

func replaceBadWords(params ChirpParameters) []string {
	words := strings.Split(params.Body, " ")
	badWords := []string{"kerfuffle",
		"sharbert",
		"fornax"}
	for i, word := range words {
		for _, badWord := range badWords {
			if strings.EqualFold(word, badWord) {
				words[i] = "****"
				continue
			}
		}
	}
	return words
}
