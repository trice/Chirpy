package main

import (
    "net/http"
)

func HandleHealthz(writer http.ResponseWriter, request *http.Request)  {
    writer.Header().Set("Content-Type", "text/plain; charset=utf-8") // normal header
    writer.WriteHeader(http.StatusOK)
    writer.Write([]byte("OK"))
}


func main() {
    serveMux := http.NewServeMux()
    server := http.Server {
        Handler: serveMux,
        Addr: ":8080",
    }
    serveMux.Handle("/app/", http.StripPrefix("/app", http.FileServer(http.Dir("."))))
    serveMux.HandleFunc("/healthz", HandleHealthz)
    server.ListenAndServe()
}
