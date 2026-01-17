package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		ip := r.Header.Get("x-forwarded-for")
		if ip == "" {
			ip = r.RemoteAddr
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(ip + "\n"))
	})
	handler := mux
	log.Println("Starting server on :" + port)
	http.ListenAndServe(":"+port, handler)
}
