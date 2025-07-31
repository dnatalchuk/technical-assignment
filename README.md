# Real-Time Multi-Tenant Event Feed

This project provides a simple real-time event broadcasting system implemented in Go.

## Features

- WebSocket server with tenant isolation
- REST endpoint `POST /events` for publishing events
- Basic HTML frontend in `frontend/` demonstrating usage
- In-memory storage only

## Prerequisites

- Go 1.20 or newer. See <https://go.dev/doc/install> for installation instructions.
- No additional dependencies are required. The provided tests can be run with `go test ./...`.

## Running

```
cd backend
go run .
```

Open `frontend/index.html` in your browser. Use two browser windows with different tenants to verify that events are isolated.

## Testing

```
cd backend
go test ./...
```
