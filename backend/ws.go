package main

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
)

const magicKey = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// wsConn implements minimal WebSocket connection for server->client messages

type wsConn struct {
	c  net.Conn
	mu sync.Mutex
}

func newWSConn(c net.Conn) *wsConn {
	return &wsConn{c: c}
}

func (w *wsConn) WriteJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return w.writeFrame(1, data) // text frame opcode=1
}

func (w *wsConn) writeFrame(opcode byte, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	header := []byte{0x80 | opcode, 0}
	l := len(payload)
	if l < 126 {
		header[1] = byte(l)
	} else if l <= 65535 {
		header[1] = 126
		ext := make([]byte, 2)
		binary.BigEndian.PutUint16(ext, uint16(l))
		header = append(header, ext...)
	} else {
		header[1] = 127
		ext := make([]byte, 8)
		binary.BigEndian.PutUint64(ext, uint64(l))
		header = append(header, ext...)
	}
	if _, err := w.c.Write(header); err != nil {
		return err
	}
	_, err := w.c.Write(payload)
	return err
}

func (w *wsConn) readLoop(onClose func()) {
	buf := make([]byte, 2)
	for {
		if _, err := io.ReadFull(w.c, buf); err != nil {
			break
		}
		fin := buf[0]&0x80 != 0
		opcode := buf[0] & 0x0F
		masked := buf[1]&0x80 != 0
		length := int(buf[1] & 0x7F)
		if length == 126 {
			ext := make([]byte, 2)
			if _, err := io.ReadFull(w.c, ext); err != nil {
				break
			}
			length = int(binary.BigEndian.Uint16(ext))
		} else if length == 127 {
			ext := make([]byte, 8)
			if _, err := io.ReadFull(w.c, ext); err != nil {
				break
			}
			length = int(binary.BigEndian.Uint64(ext))
		}
		maskKey := make([]byte, 4)
		if masked {
			if _, err := io.ReadFull(w.c, maskKey); err != nil {
				break
			}
		}
		payload := make([]byte, length)
		if _, err := io.ReadFull(w.c, payload); err != nil {
			break
		}
		if masked {
			for i := range payload {
				payload[i] ^= maskKey[i%4]
			}
		}
		if opcode == 8 { // close frame
			break
		}
		if !fin {
			// ignore fragmented frames for simplicity
			continue
		}
		// ignore payload for now
	}
	onClose()
}

func (w *wsConn) Close() error {
	return w.c.Close()
}

// serveWS handles WebSocket upgrade and connection registration
func serveWS(hub *EventHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.URL.Query().Get("tenant")
		if tenantID == "" {
			http.Error(w, "missing tenant", http.StatusBadRequest)
			return
		}
		if !headerContains(r.Header, "Connection", "upgrade") ||
			!headerContains(r.Header, "Upgrade", "websocket") {
			http.Error(w, "not websocket", http.StatusBadRequest)
			return
		}
		key := r.Header.Get("Sec-WebSocket-Key")
		if key == "" {
			http.Error(w, "missing key", http.StatusBadRequest)
			return
		}
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "cannot hijack", http.StatusInternalServerError)
			return
		}
		netConn, buf, err := hijacker.Hijack()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if buf.Reader.Buffered() > 0 {
			log.Println("unexpected buffered data")
		}
		accept := computeAcceptKey(key)
		resp := "HTTP/1.1 101 Switching Protocols\r\n" +
			"Upgrade: websocket\r\n" +
			"Connection: Upgrade\r\n" +
			"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
		if _, err := netConn.Write([]byte(resp)); err != nil {
			netConn.Close()
			return
		}
		ws := newWSConn(netConn)
		hub.registerConn(tenantID, ws)
		go ws.readLoop(func() {
			hub.unregisterConn(tenantID, ws)
		})
	}
}

func computeAcceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + magicKey))
	sum := h.Sum(nil)
	return base64.StdEncoding.EncodeToString(sum)
}

func headerContains(h http.Header, name, value string) bool {
	for _, v := range h.Values(name) {
		if strings.EqualFold(strings.TrimSpace(v), value) {
			return true
		}
	}
	return false
}
