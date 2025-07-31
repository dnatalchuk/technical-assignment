package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

func main() {
	hub := newEventHub()
	http.HandleFunc("/ws", serveWS(hub))
	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			http.Error(w, "missing tenant header", http.StatusBadRequest)
			return
		}
		body, err := ioutil.ReadAll(r.Body)
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
		log.Printf("tenant %s: event posted: %s", tenantID, req.Message)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(e)
	})
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
