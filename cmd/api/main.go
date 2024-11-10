package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/DeabLabs/urlmd/pkg/converter"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type server struct {
	converter *converter.Converter
}

type ConvertRequest struct {
	URL string `json:"url"`
}

type ConvertResponse struct {
	Markdown    string    `json:"markdown"`
	LastFetched time.Time `json:"last_fetched"`
}

func main() {
	// Get port from environment variable, default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize converter
	conv, err := converter.NewConverter(converter.Config{
		CacheDuration: 24 * time.Hour,
		CachePath:     "cache.db",
		Timeout:       30 * time.Second,
		UserAgent:     "Webpage-To-Markdown-Bot/1.0",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer conv.Close()

	// Initialize server
	s := &server{converter: conv}

	// Setup router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Routes
	r.Post("/convert", s.handleConvert)
	r.Get("/health", s.handleHealth)

	// Start server
	addr := fmt.Sprintf(":%s", port)
	log.Printf("Server starting on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

func (s *server) handleConvert(w http.ResponseWriter, r *http.Request) {
	var req ConvertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	markdown, err := s.converter.Convert(r.Context(), req.URL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := ConvertResponse{
		Markdown:    markdown,
		LastFetched: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
