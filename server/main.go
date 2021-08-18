package main

import (
	"embed"
	"log"
	"net/http"
	"os"
)

//go:embed public
var publicFS embed.FS

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.Handle("/public/", http.FileServer(http.FS(publicFS)))
	http.Handle("/", MessageMiddleware(http.HandlerFunc(handleIndex)))
	http.HandleFunc("/contact-form", handleContactForm)
	log.Printf("Server running on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
