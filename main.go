package main

import (
	"fmt"
	"net/http"
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
    writer.Header().Set("Content-Type", "text/plain; charset=utf-8") // normal header
    writer.WriteHeader(http.StatusOK)
    hits := cfg.fileserverHits.Load()
    message := fmt.Sprintf("Hits: %v", hits)
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


func main() {
    theCounter := apiConfig{}
    serveMux := http.NewServeMux()
    server := http.Server {
        Handler: serveMux,
        Addr: ":8080",
    }
    serveMux.Handle("/app/", http.StripPrefix("/app",
        theCounter.MiddlewareMetricsInc(http.FileServer(http.Dir(".")))))
    serveMux.HandleFunc("GET /healthz", HandleHealthz)
    serveMux.HandleFunc("GET /metrics", theCounter.GetHits)
    serveMux.HandleFunc("POST /reset", theCounter.resetHits)

    server.ListenAndServe()
}
