package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"testing"
)

// fakeConn records written messages

type fakeConn struct {
	mu   sync.Mutex
	msgs []Event
}

func (f *fakeConn) WriteJSON(v interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	var e Event
	b, _ := json.Marshal(v)
	json.Unmarshal(b, &e)
	f.msgs = append(f.msgs, e)
	return nil
}

func (f *fakeConn) Close() error { return nil }

type errConn struct{ closed bool }

func (e *errConn) WriteJSON(v interface{}) error { return errors.New("boom") }

func (e *errConn) Close() error {
	e.closed = true
	return nil
}

func TestTenantIsolation(t *testing.T) {
	hub := newEventHub()
	cA := &fakeConn{}
	cB := &fakeConn{}

	hub.registerConn("tenantA", cA)
	hub.registerConn("tenantB", cB)

	hub.postEvent("tenantA", "hello A")
	hub.postEvent("tenantB", "hello B")

	if len(cA.msgs) != 1 || cA.msgs[0].Message != "hello A" {
		t.Fatalf("tenant A should receive its event")
	}
	if len(cB.msgs) != 1 || cB.msgs[0].Message != "hello B" {
		t.Fatalf("tenant B should receive its event")
	}
}

func TestFailingConnectionRemoval(t *testing.T) {
	hub := newTenantHub()
	c := &errConn{}
	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(orig)

	hub.addConn(c)
	_ = hub.addEvent(Event{TenantID: "t1"})

	hub.mu.Lock()
	_, exists := hub.connections[c]
	hub.mu.Unlock()
	if exists {
		t.Fatalf("connection should be removed after failure")
	}
	if !c.closed {
		t.Fatalf("connection should be closed")
	}
	if !strings.Contains(buf.String(), "failed to write event") {
		t.Fatalf("expected log message for write failure")
	}
}

func TestEventHistoryLimit(t *testing.T) {
	hub := newTenantHub()
	for i := 0; i < maxEvents+10; i++ {
		_ = hub.addEvent(Event{TenantID: "t1", Message: fmt.Sprintf("%d", i)})
	}
	hub.mu.Lock()
	count := len(hub.events)
	first := hub.events[0].Message
	last := hub.events[len(hub.events)-1].Message
	hub.mu.Unlock()
	if count != maxEvents {
		t.Fatalf("expected %d events, got %d", maxEvents, count)
	}
	if first != "10" {
		t.Fatalf("expected oldest message to be '10', got %s", first)
	}
	if last != fmt.Sprintf("%d", maxEvents+9) {
		t.Fatalf("expected last message to be %d, got %s", maxEvents+9, last)
	}
}

func TestPostEventSetsElapsed(t *testing.T) {
	hub := newEventHub()
	e := hub.postEvent("tenant1", "msg")
	if e.Elapsed == "" {
		t.Fatalf("expected elapsed to be set")
	}

	hub.mu.Lock()
	stored := hub.tenants["tenant1"].events[0].Elapsed
	hub.mu.Unlock()
	if stored == "" {
		t.Fatalf("stored event should have elapsed set")
	}
	if stored != e.Elapsed {
		t.Fatalf("elapsed mismatch: %s vs %s", stored, e.Elapsed)
	}
}

func BenchmarkPostEvent(b *testing.B) {
	hub := newEventHub()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.postEvent("bench", "msg")
	}
}
