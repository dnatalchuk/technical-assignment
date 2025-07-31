package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func setupTestServer() (*httptest.Server, *EventHub) {
	hub := newEventHub()
	mux := http.NewServeMux()
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
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		e := hub.postEvent(tenantID, req.Message)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(e)
	})
	srv := httptest.NewServer(mux)
	return srv, hub
}

type wsClient struct {
	c net.Conn
	r *bufio.Reader
}

func dialWS(rawurl string) (*wsClient, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	conn, err := net.Dial("tcp", u.Host)
	if err != nil {
		return nil, err
	}
	keyBytes := make([]byte, 16)
	rand.Read(keyBytes)
	key := base64.StdEncoding.EncodeToString(keyBytes)
	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: %s\r\n\r\n", u.RequestURI(), u.Host, key)
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, err
	}
	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, err
	}
	if !strings.Contains(status, "101") {
		conn.Close()
		return nil, fmt.Errorf("handshake failed: %s", strings.TrimSpace(status))
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, err
		}
		if line == "\r\n" {
			break
		}
	}
	return &wsClient{c: conn, r: reader}, nil
}

func (w *wsClient) ReadJSON(v interface{}, deadline time.Duration) error {
	if deadline > 0 {
		w.c.SetReadDeadline(time.Now().Add(deadline))
	} else {
		w.c.SetReadDeadline(time.Time{})
	}
	header := make([]byte, 2)
	if _, err := io.ReadFull(w.r, header); err != nil {
		return err
	}
	length := int(header[1] & 0x7F)
	if length == 126 {
		ext := make([]byte, 2)
		if _, err := io.ReadFull(w.r, ext); err != nil {
			return err
		}
		length = int(binary.BigEndian.Uint16(ext))
	} else if length == 127 {
		ext := make([]byte, 8)
		if _, err := io.ReadFull(w.r, ext); err != nil {
			return err
		}
		length = int(binary.BigEndian.Uint64(ext))
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(w.r, payload); err != nil {
		return err
	}
	return json.Unmarshal(payload, v)
}

func (w *wsClient) Close() error { return w.c.Close() }

func postEvent(t *testing.T, client *http.Client, url, tenant, msg string) Event {
	body := bytes.NewBufferString(fmt.Sprintf(`{"message":"%s"}`, msg))
	req, err := http.NewRequest(http.MethodPost, url+"/events", body)
	if err != nil {
		t.Fatalf("postEvent: %v", err)
	}
	req.Header.Set("X-Tenant-ID", tenant)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("postEvent: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	var e Event
	if err := json.NewDecoder(resp.Body).Decode(&e); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return e
}

func TestEventsEndpoint(t *testing.T) {
	srv, _ := setupTestServer()
	defer srv.Close()
	client := srv.Client()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/events", bytes.NewBufferString(`{"message":"x"}`))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	e1 := postEvent(t, client, srv.URL, "tenant1", "hello1")
	if e1.TenantID != "tenant1" || e1.Message != "hello1" || e1.ID == "" || e1.Timestamp.IsZero() {
		t.Fatalf("invalid event %+v", e1)
	}
	if e1.Elapsed == "" {
		t.Fatalf("expected elapsed to be set")
	}
	e2 := postEvent(t, client, srv.URL, "tenant2", "hello2")
	if e2.TenantID != "tenant2" || e2.Message != "hello2" {
		t.Fatalf("invalid event %+v", e2)
	}
	if e2.Elapsed == "" {
		t.Fatalf("expected elapsed to be set")
	}
}

func TestWebsocketTenantIsolation(t *testing.T) {
	srv, _ := setupTestServer()
	defer srv.Close()
	client := srv.Client()

	if _, err := dialWS(srv.URL + "/ws"); err == nil {
		t.Fatalf("connection without tenant should fail")
	}

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

	postEvent(t, client, srv.URL, "tenantA", "msgA")

	var ev Event
	if err := wsA.ReadJSON(&ev, time.Second); err != nil {
		t.Fatalf("read tenantA: %v", err)
	}
	if ev.Message != "msgA" || ev.TenantID != "tenantA" {
		t.Fatalf("unexpected event %+v", ev)
	}
	if err := wsB.ReadJSON(&ev, 200*time.Millisecond); err == nil {
		t.Fatalf("tenantB should not receive event")
	}

	postEvent(t, client, srv.URL, "tenantB", "msgB")
	if err := wsB.ReadJSON(&ev, time.Second); err != nil {
		t.Fatalf("read tenantB: %v", err)
	}
	if ev.Message != "msgB" || ev.TenantID != "tenantB" {
		t.Fatalf("unexpected event %+v", ev)
	}
	if err := wsA.ReadJSON(&ev, 200*time.Millisecond); err == nil {
		t.Fatalf("tenantA should not receive event from B")
	}
}
func TestServeWSValidation(t *testing.T) {
	hub := newEventHub()
	srv := httptest.NewServer(serveWS(hub))
	defer srv.Close()
	client := srv.Client()

	testCases := []struct {
		name    string
		path    string
		headers map[string]string
		expect  string
	}{
		{
			name: "missing tenant",
			path: "/ws",
			headers: map[string]string{
				"Connection":            "Upgrade",
				"Upgrade":               "websocket",
				"Sec-WebSocket-Version": "13",
				"Sec-WebSocket-Key":     "abc",
			},
			expect: "missing tenant",
		},
		{
			name: "missing connection",
			path: "/ws?tenant=t1",
			headers: map[string]string{
				"Upgrade":               "websocket",
				"Sec-WebSocket-Version": "13",
				"Sec-WebSocket-Key":     "abc",
			},
			expect: "not websocket",
		},
		{
			name: "missing upgrade",
			path: "/ws?tenant=t1",
			headers: map[string]string{
				"Connection":            "Upgrade",
				"Sec-WebSocket-Version": "13",
				"Sec-WebSocket-Key":     "abc",
			},
			expect: "not websocket",
		},
		{
			name: "missing key",
			path: "/ws?tenant=t1",
			headers: map[string]string{
				"Connection":            "Upgrade",
				"Upgrade":               "websocket",
				"Sec-WebSocket-Version": "13",
			},
			expect: "missing key",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, srv.URL+tc.path, nil)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("do request: %v", err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", resp.StatusCode)
			}
			if !strings.Contains(string(body), tc.expect) {
				t.Fatalf("expected body to contain %q, got %q", tc.expect, string(body))
			}
		})
	}
}
