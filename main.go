package main

import (
    "net/http"
)

func main() {
    serveMux := http.NewServeMux()
    server := http.Server {
        Handler: serveMux,
        Addr: ":8080",
    }
    serveMux.Handle("/", http.FileServer(http.Dir(".")))
    server.ListenAndServe()
}
