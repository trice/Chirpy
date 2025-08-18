package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
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

func (cfg * apiConfig) resetHits(writer http.ResponseWriter, request *http.Request) {
    cfg.fileserverHits.Store(0)
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
    theCounter := apiConfig{}
    serveMux := http.NewServeMux()
    server := http.Server {
        Handler: serveMux,
        Addr: ":8080",
    }
    serveMux.Handle("/app/", http.StripPrefix("/app",
        theCounter.MiddlewareMetricsInc(http.FileServer(http.Dir(".")))))
    serveMux.HandleFunc("GET /api/healthz", HandleHealthz)
    serveMux.HandleFunc("GET /admin/metrics", theCounter.GetHits)
    serveMux.HandleFunc("POST /admin/reset", theCounter.resetHits)
    serveMux.HandleFunc("POST /api/validate_chirp", func(w http.ResponseWriter, r *http.Request) {
        // decode the body to a struct and then check the length of the string
        type body struct {
            Body string `json:"body"`
        }
        data, err := io.ReadAll(r.Body)
        if err != nil {
            w.Header().Set("Content-Type", "application/json; charset=utf-8")
            w.WriteHeader(400)
            w.Write([]byte(`{"error":"something went wrong"}`))
            return
        }

        rb := body{}
        err = json.Unmarshal(data, &rb)
        if err != nil {
            w.Header().Set("Content-Type", "application/json; charset=utf-8")
            w.WriteHeader(400)
            w.Write([]byte(`{"error":"something went wrong"}`))
            return
        }

        rb.Body = scrubMessage(rb.Body)

        if len(rb.Body) > 140 {
            w.Header().Set("Content-Type", "application/json; charset=utf-8")
            w.WriteHeader(400)
            w.Write([]byte(`{"error": "Chirp is too long"}`))
            return
        } else {
            type respBod struct {
                CleanedBody string `json:"cleaned_body"`
            }
            rbod := respBod {
                CleanedBody: rb.Body,
            }
            d, _ := json.Marshal(rbod)
            w.Header().Set("Content-Type", "application/json; charset=utf-8")
            w.WriteHeader(200)
            w.Write(d)
            return
        }
    })
    server.ListenAndServe()
}
