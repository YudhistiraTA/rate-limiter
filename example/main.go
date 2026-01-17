package main

import (
	"fmt"
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
		forwardedForStr := fmt.Sprintf("x-forwarded-for: %s", r.Header.Get("x-forwarded-for"))
		ip := fmt.Sprintf("ip: %s", r.RemoteAddr)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(forwardedForStr + "\n" + ip + "\n"))
	})
	handler := mux
	log.Println("Starting server on :" + port)
	http.ListenAndServe(":"+port, handler)
}
