package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Server struct {
	mu         sync.Mutex
	data       map[string]string
	requests   int
	shutdownCh chan struct{}
	wg         sync.WaitGroup
}

func NewServer() *Server {
	return &Server{
		data:       make(map[string]string),
		shutdownCh: make(chan struct{}),
	}
}

func (s *Server) postDataHandler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	var entry map[string]string
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	for k, v := range entry {
		s.data[k] = v
	}
	s.requests++
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) getDataHandler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if err := json.NewEncoder(w).Encode(s.data); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
	s.requests++
}

func (s *Server) statsHandler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	stats := map[string]int{"requests": s.requests}
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
	s.requests++
}

func (s *Server) deleteDataHandler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	key := strings.TrimPrefix(r.URL.Path, "/data/")
	if key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}
	
	delete(s.data, key)
	s.requests++
	w.WriteHeader(http.StatusOK)
}

func (s *Server) startBackgroundWorker() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				s.mu.Lock()
				log.Printf("Status: %d requests, %d keys", s.requests, len(s.data))
				s.mu.Unlock()
			case <-s.shutdownCh:
				log.Println("Background worker stopping")
				return
			}
		}
	}()
}

func (s *Server) shutdown() {
	close(s.shutdownCh)
	s.wg.Wait()
}

func main() {
	fmt.Println("Сервер запускается...")

	srv := NewServer()
	srv.startBackgroundWorker()

	http.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			srv.postDataHandler(w, r)
		case http.MethodGet:
			srv.getDataHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	http.HandleFunc("/data/", srv.deleteDataHandler)
	http.HandleFunc("/stats", srv.statsHandler)

	server := &http.Server{Addr: ":8080"}
	go func() {
		log.Println("Server is running on http://localhost:8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %s", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	srv.shutdown()
	log.Println("Server stopped gracefully")
}
