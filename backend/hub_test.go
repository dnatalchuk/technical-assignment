package main

import (
	"encoding/json"
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
