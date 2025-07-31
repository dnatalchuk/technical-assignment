package main

import (
	"net/http"
	"testing"
)

func TestHeaderContains(t *testing.T) {
	h := http.Header{}
	h.Add("Connection", " keep-alive ")
	h.Add("Connection", " UpGrade ")
	h.Add("Upgrade", " WebSocket ")

	if !headerContains(h, "Connection", "upgrade") {
		t.Fatalf("expected headerContains to match Upgrade in Connection")
	}
	if !headerContains(h, "Upgrade", "websocket") {
		t.Fatalf("expected headerContains to match websocket in Upgrade")
	}
	if headerContains(h, "Upgrade", "h2c") {
		t.Fatalf("did not expect h2c in Upgrade")
	}
}
