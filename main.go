package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/trice/Chirpy/internal/database"
)

type apiConfig struct {
	fileserverHits atomic.Int32
    queries *database.Queries
    platform string
}

func (cfg *apiConfig) MiddlewareMetricsInc(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        cfg.fileserverHits.Add(1)
        next.ServeHTTP(w, r)
    })
}

func (cfg * apiConfig) GetHits(writer http.ResponseWriter, request *http.Request) {
    htmlFormat := `<html>
                      <body>
                        <h1>Welcome, Chirpy Admin</h1>
                        <p>Chirpy has been visited %d times!</p>
                      </body>
                    </html>`
    writer.Header().Set("Content-Type", "text/html; charset=utf-8") // normal header
    writer.WriteHeader(http.StatusOK)
    hits := cfg.fileserverHits.Load()
    message := fmt.Sprintf(htmlFormat, hits)
    writer.Write([]byte(message))
}

func (cfg * apiConfig) createUser(writer http.ResponseWriter, request *http.Request) {
    // decode the body to a struct and then check the length of the string
    type body struct {
        Email string `json:"email"`
    }
    data, err := io.ReadAll(request.Body)
    if err != nil {
        writer.Header().Set("Content-Type", "application/json; charset=utf-8")
        writer.WriteHeader(http.StatusBadRequest)
        writer.Write([]byte(`{"error":"something went wrong"}`))
        return
    }

    rb := body{}
    err = json.Unmarshal(data, &rb)
    if err != nil {
        writer.Header().Set("Content-Type", "application/json; charset=utf-8")
        writer.WriteHeader(http.StatusBadRequest)
        writer.Write([]byte(`{"error":"something went wrong"}`))
        return
    }
    user, err := cfg.queries.CreateUser(request.Context(), rb.Email)
    if err != nil {
        writer.Header().Set("Content-Type", "application/json; charset=utf-8")
        writer.WriteHeader(http.StatusBadRequest)
        writer.Write([]byte(`{"error":"something went wrong"}`))
        return
    }

    d, _ := json.Marshal(user)
    writer.Header().Set("Content-Type", "application/json; charset=utf-8")
    writer.WriteHeader(http.StatusCreated)
    writer.Write(d)
}

func (cfg * apiConfig) resetHits(writer http.ResponseWriter, request *http.Request) {
    cfg.fileserverHits.Store(0)
    if cfg.platform == "dev" {
        cfg.queries.DeleteUser(request.Context())
    } else {
        writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
        writer.WriteHeader(http.StatusForbidden)
    }
}

func (cfg * apiConfig) createChirp(w http.ResponseWriter, r *http.Request) {
    // decode the body to a struct and then check the length of the string
    type body struct {
        Body string `json:"body"`
        UserId uuid.UUID `json:"user_id"`
    }

    data, err := io.ReadAll(r.Body)
    if err != nil {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.WriteHeader(http.StatusBadRequest)
        w.Write([]byte(`{"error":"something went wrong"}`))
        return
    }

    rb := body{}
    err = json.Unmarshal(data, &rb)
    if err != nil {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.WriteHeader(http.StatusBadRequest)
        w.Write([]byte(`{"error":"something went wrong"}`))
        return
    }

    rb.Body = scrubMessage(rb.Body)

    if len(rb.Body) > 140 {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.WriteHeader(http.StatusBadRequest)
        w.Write([]byte(`{"error": "Chirp is too long"}`))
        return
    } else {
        newChirp := database.CreateChirpParams {
            Body: rb.Body,
            UserID: rb.UserId,
        }
        dbResult, err := cfg.queries.CreateChirp(r.Context(), newChirp)
        if err != nil {
            w.Header().Set("Content-Type", "application/json; charset=utf-8")
            w.WriteHeader(http.StatusBadRequest)
            w.Write([]byte(`{"error":"something went wrong"}`))
            return
        }
        d, _ := json.Marshal(dbResult)
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.WriteHeader(http.StatusCreated)
        w.Write(d)
        return
    }
}

func (cfg *apiConfig) getChirps(w http.ResponseWriter, r *http.Request)  {
    chirpsAscByCreate, err := cfg.queries.GetChirps(r.Context())
    if err != nil {
        http.Error(w, "Error reading chirps", http.StatusNotFound)
    }

    d, _ := json.Marshal(chirpsAscByCreate)
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    w.Write(d)
}

func (cfg* apiConfig) getChirpBy(w http.ResponseWriter, r *http.Request)  {
    var chirpId uuid.NullUUID
    chirpId.UUID = uuid.MustParse(r.PathValue("chirpID"))
    chirpId.Valid = true
    chirpResult, err := cfg.queries.GetChirpById(r.Context(), chirpId)
    if err != nil {
        http.Error(w, "chirp not found", http.StatusNotFound)
        return
    }

    d, _ := json.Marshal(chirpResult)
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    w.Write(d)
}

func HandleHealthz(writer http.ResponseWriter, request *http.Request)  {
    writer.Header().Set("Content-Type", "text/plain; charset=utf-8") // normal header
    writer.WriteHeader(http.StatusOK)
    writer.Write([]byte("OK"))
}

func scrubMessage(message string) string  {
    tmpWords := strings.Split(message, " ")
    for i, word := range tmpWords {
        tmpWord := strings.ToLower(word)
        switch tmpWord {
        case "kerfuffle", "sharbert", "fornax":
            tmpWords[i] = "****"
        default:
        }
    }
    return strings.Join(tmpWords, " ")
}

func main() {
    godotenv.Load()
    dbURL := os.Getenv("DB_URL")
    platform := os.Getenv("PLATFORM")

    db, err := sql.Open("postgres", dbURL)
    if err != nil {
        fmt.Printf("database open failed")
        return
    }
    dbQueries := database.New(db)

    theCounter := apiConfig{}
    theCounter.queries = dbQueries
    theCounter.platform = platform

    serveMux := http.NewServeMux()
    server := http.Server {
        Handler: serveMux,
        Addr: ":8080",
    }

    serveMux.Handle("/app/", http.StripPrefix("/app",
        theCounter.MiddlewareMetricsInc(http.FileServer(http.Dir(".")))))

    serveMux.HandleFunc("GET /api/healthz", HandleHealthz)
    serveMux.HandleFunc("GET /admin/metrics", theCounter.GetHits)
    serveMux.HandleFunc("POST /api/users", theCounter.createUser)
    serveMux.HandleFunc("POST /admin/reset", theCounter.resetHits)
    serveMux.HandleFunc("POST /api/chirps", theCounter.createChirp)
    serveMux.HandleFunc("GET /api/chirps", theCounter.getChirps)
    serveMux.HandleFunc("GET /api/chirps/{chirpID}", theCounter.getChirpBy)

    server.ListenAndServe()
}
