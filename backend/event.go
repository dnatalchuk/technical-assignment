package main

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"time"
)

// Event represents a single event message
type Event struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// newEvent creates a new event with generated ID and current timestamp
func newEvent(tenantID, message string) Event {
	return Event{
		ID:        generateID(),
		TenantID:  tenantID,
		Message:   message,
		Timestamp: time.Now().UTC().Truncate(time.Second),
	}
}

func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		log.Printf("generateID failed: %v", err)
		return ""
	}
	return hex.EncodeToString(b)
}
