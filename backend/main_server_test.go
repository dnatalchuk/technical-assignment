package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMainHTTPServer(t *testing.T) {
	srv := httptest.NewServer(newServer())
	defer srv.Close()
	client := srv.Client()

	// missing tenant header
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/events", bytes.NewBufferString(`{"message":"x"}`))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	wsA, err := dialWS(srv.URL + "/ws?tenant=tenantA")
	if err != nil {
		t.Fatalf("dial tenantA: %v", err)
	}
	defer wsA.Close()
	wsB, err := dialWS(srv.URL + "/ws?tenant=tenantB")
	if err != nil {
		t.Fatalf("dial tenantB: %v", err)
	}
	defer wsB.Close()

	postEvent(t, client, srv.URL, "tenantA", "a1")

	var ev Event
	if err := wsA.ReadJSON(&ev, time.Second); err != nil {
		t.Fatalf("read tenantA: %v", err)
	}
	if ev.Message != "a1" || ev.TenantID != "tenantA" {
		t.Fatalf("unexpected event %+v", ev)
	}
	if err := wsB.ReadJSON(&ev, 200*time.Millisecond); err == nil {
		t.Fatalf("tenantB should not receive tenantA event")
	}

	postEvent(t, client, srv.URL, "tenantB", "b1")
	if err := wsB.ReadJSON(&ev, time.Second); err != nil {
		t.Fatalf("read tenantB: %v", err)
	}
	if ev.Message != "b1" || ev.TenantID != "tenantB" {
		t.Fatalf("unexpected event %+v", ev)
	}
	if err := wsA.ReadJSON(&ev, 200*time.Millisecond); err == nil {
		t.Fatalf("tenantA should not receive tenantB event")
	}
}

func TestEventsMethodNotAllowed(t *testing.T) {
	srv := httptest.NewServer(newServer())
	defer srv.Close()
	client := srv.Client()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/events", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestEventsBadJSON(t *testing.T) {
	srv := httptest.NewServer(newServer())
	defer srv.Close()
	client := srv.Client()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/events", bytes.NewBufferString("{invalid"))
	req.Header.Set("X-Tenant-ID", "tenant1")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}
