package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

// helper to read all bytes from conn until closed
func readAll(c net.Conn) []byte {
	b := new(bytes.Buffer)
	io.Copy(b, c)
	return b.Bytes()
}

func TestWriteFrame(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
	}{
		{"small", []byte("hi")},
		{"medium", bytes.Repeat([]byte{'x'}, 130)},
		{"large", bytes.Repeat([]byte{'y'}, 66000)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, server := net.Pipe()
			ws := newWSConn(server)
			go func() {
				if err := ws.writeFrame(1, tc.payload); err != nil {
					t.Errorf("writeFrame error: %v", err)
				}
				server.Close()
			}()
			got := readAll(client)
			var expect bytes.Buffer
			expect.WriteByte(0x81) // FIN + text opcode
			l := len(tc.payload)
			switch {
			case l < 126:
				expect.WriteByte(byte(l))
			case l <= 65535:
				expect.WriteByte(126)
				ext := make([]byte, 2)
				binary.BigEndian.PutUint16(ext, uint16(l))
				expect.Write(ext)
			default:
				expect.WriteByte(127)
				ext := make([]byte, 8)
				binary.BigEndian.PutUint64(ext, uint64(l))
				expect.Write(ext)
			}
			expect.Write(tc.payload)
			if !bytes.Equal(got, expect.Bytes()) {
				t.Fatalf("unexpected frame bytes")
			}
			client.Close()
		})
	}
}

func sendMaskedFrame(w io.Writer, opcode byte, payload []byte) {
	mask := []byte{1, 2, 3, 4}
	l := len(payload)
	header := []byte{0x80 | opcode}
	switch {
	case l < 126:
		header = append(header, 0x80|byte(l))
	case l <= 65535:
		header = append(header, 0x80|126)
		ext := make([]byte, 2)
		binary.BigEndian.PutUint16(ext, uint16(l))
		header = append(header, ext...)
	default:
		header = append(header, 0x80|127)
		ext := make([]byte, 8)
		binary.BigEndian.PutUint64(ext, uint64(l))
		header = append(header, ext...)
	}
	w.Write(header)
	w.Write(mask)
	masked := make([]byte, l)
	for i, b := range payload {
		masked[i] = b ^ mask[i%4]
	}
	w.Write(masked)
}

func TestReadLoop(t *testing.T) {
	client, server := net.Pipe()
	ws := newWSConn(server)
	closed := make(chan struct{})
	go ws.readLoop("t1", func() { close(closed) })

	go func() {
		sendMaskedFrame(client, 1, []byte("hello"))
		sendMaskedFrame(client, 8, nil)
		client.Close()
	}()

	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("readLoop did not exit")
	}
	ws.Close()
}
