package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Server struct {
	mu         sync.Mutex
	data       map[string]string
	requests   int
	shutdownCh chan struct{}
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
	json.NewEncoder(w).Encode(s.data)
	s.requests++
}

func (s *Server) statsHandler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats := map[string]int{"requests": s.requests}
	json.NewEncoder(w).Encode(stats)
	s.requests++
}

func (s *Server) deleteDataHandler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := r.URL.Path[len("/data/"):]
	delete(s.data, key)
	s.requests++
	w.WriteHeader(http.StatusOK)
}

func (s *Server) startBackgroundWorker() {
	go func() {
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
}

func main() {
	fmt.Println("Сервер запускается...")

	srv := NewServer()
	srv.startBackgroundWorker()

	http.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			srv.postDataHandler(w, r)
		} else if r.Method == "GET" {
			srv.getDataHandler(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	http.HandleFunc("/data/", srv.deleteDataHandler)
	http.HandleFunc("/stats", srv.statsHandler)

	go func() {
		log.Println("Server is running on http://localhost:8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("Server failed: %s", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	log.Println("Shutting down server...")
	srv.shutdown()
	time.Sleep(1 * time.Second)
}
