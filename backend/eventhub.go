package main

import (
	"log"
	"sync"
	"time"
)

const maxEvents = 1000

// Conn defines minimal methods for a websocket connection
// real websocket or mock must implement this interface
// WriteJSON should serialize v as JSON and send
// Close closes the connection
// in our simple implementation, only text frames with JSON will be used

type Conn interface {
	WriteJSON(v interface{}) error
	Close() error
}

// TenantHub manages events and connections for a single tenant
type TenantHub struct {
	events      []Event
	connections map[Conn]bool
	mu          sync.Mutex
}

func newTenantHub() *TenantHub {
	return &TenantHub{
		events:      make([]Event, 0, maxEvents),
		connections: make(map[Conn]bool),
	}
}

// addEvent stores the event, broadcasts it, and returns the stored event with the Elapsed field populated
func (h *TenantHub) addEvent(e Event) Event {
	start := time.Now()
	h.mu.Lock()
	if len(h.events) >= maxEvents {
		h.events = h.events[1:]
	}
	h.events = append(h.events, e)
	idx := len(h.events) - 1

	conns := make([]Conn, 0, len(h.connections))
	for c := range h.connections {
		conns = append(conns, c)
	}
	h.mu.Unlock()

	for _, c := range conns {
		if err := c.WriteJSON(e); err != nil {
			log.Printf("tenant %s: failed to write event: %v", e.TenantID, err)
			h.mu.Lock()
			delete(h.connections, c)
			h.mu.Unlock()
			if err := c.Close(); err != nil {
				log.Printf("tenant %s: failed to close connection: %v", e.TenantID, err)
			}
		}
	}

	elapsed := time.Since(start).String()
	h.mu.Lock()
	h.events[idx].Elapsed = elapsed
	h.mu.Unlock()
	e.Elapsed = elapsed

	return e
}

// addConn registers a new connection
func (h *TenantHub) addConn(c Conn) {
	h.mu.Lock()
	h.connections[c] = true
	h.mu.Unlock()
}

// removeConn removes a connection
func (h *TenantHub) removeConn(c Conn) {
	h.mu.Lock()
	if _, ok := h.connections[c]; ok {
		delete(h.connections, c)
		if err := c.Close(); err != nil {
			log.Printf("failed to close connection: %v", err)
		}
	}
	h.mu.Unlock()
}

// EventHub manages tenants
type EventHub struct {
	tenants map[string]*TenantHub
	mu      sync.Mutex
}

func newEventHub() *EventHub {
	return &EventHub{tenants: make(map[string]*TenantHub)}
}

// postEvent creates and stores event for tenant
func (h *EventHub) postEvent(tenantID, message string) Event {
	h.mu.Lock()
	tenant := h.ensureTenant(tenantID)
	h.mu.Unlock()
	e := newEvent(tenantID, message)
	return tenant.addEvent(e)
}

func (h *EventHub) ensureTenant(id string) *TenantHub {
	if t, ok := h.tenants[id]; ok {
		return t
	}
	t := newTenantHub()
	h.tenants[id] = t
	return t
}

// registerConn registers connection to tenant
func (h *EventHub) registerConn(tenantID string, c Conn) {
	h.mu.Lock()
	tenant := h.ensureTenant(tenantID)
	h.mu.Unlock()
	tenant.addConn(c)
}

// unregisterConn removes connection from tenant
func (h *EventHub) unregisterConn(tenantID string, c Conn) {
	h.mu.Lock()
	tenant := h.tenants[tenantID]
	h.mu.Unlock()
	if tenant != nil {
		tenant.removeConn(c)
	}
}
