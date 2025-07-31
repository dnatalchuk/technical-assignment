package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"path/filepath"
)

func init() {
	// Include microseconds and UTC in log output for clearer timestamps
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.LUTC)
}

func newServer() http.Handler {
	hub := newEventHub()
	mux := http.NewServeMux()

	frontendDir := filepath.Join("..", "frontend")
	fs := http.FileServer(http.Dir(frontendDir))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))
	mux.Handle("/", fs)
	mux.HandleFunc("/ws", serveWS(hub))
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
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
		log.Printf("tenant %s: event posted: %s (took %s)", tenantID, req.Message, e.Elapsed)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(e)
	})
	return mux
}

func main() {
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", newServer()))
}
