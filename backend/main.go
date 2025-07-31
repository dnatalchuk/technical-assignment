package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"time"
)

func init() {
	// Include microseconds and UTC in log output for clearer timestamps
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.LUTC)
}

func main() {
	hub := newEventHub()
	frontendDir := filepath.Join("..", "frontend")
	fs := http.FileServer(http.Dir(frontendDir))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.Handle("/", fs)
	http.HandleFunc("/ws", serveWS(hub))
	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			http.Error(w, "missing tenant header", http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		var req struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			log.Printf("tenant %s: json parse error: %v", tenantID, err)
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		e := hub.postEvent(tenantID, req.Message)
		duration := time.Since(start)
		log.Printf("tenant %s: event posted: %s (took %v)", tenantID, req.Message, duration)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(e)
	})
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
