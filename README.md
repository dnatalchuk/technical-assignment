# Real-Time Multi-Tenant Event Feed

This project provides a simple real-time event broadcasting system implemented in Go.

## Features

- WebSocket server with tenant isolation
- REST endpoint `POST /events` for publishing events
- Basic HTML frontend in `frontend/` demonstrating usage
- In-memory storage only

## Running

```
cd backend
go run .
```

Navigate to `http://localhost:8080` in your browser. Use two browser windows with different tenants to verify that events are isolated.

## Testing

```
cd backend
go test ./...
```
