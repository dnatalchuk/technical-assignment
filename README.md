# Real-Time Multi-Tenant Event Feed

This project provides a simple real-time event broadcasting system implemented in Go.

## Problem Statement

The application must broadcast events in real time for multiple tenants while ensuring data isolation. Clients send events via a REST endpoint and listen for updates through WebSocket connections. Each connection is authenticated with the tenant identifier so that events are only delivered to the correct tenant.

### Tenant Isolation Implementation

The backend maintains an `EventHub` containing a map of tenant IDs to their own `TenantHub`. When an HTTP request posts an event, the `X-Tenant-ID` header selects the correct hub and only connections registered to that tenant receive the broadcast. WebSocket connections include `?tenant=ID` during the handshake so they can be registered to the appropriate hub. Because each tenant has separate lists of connections and events, messages never cross tenants, and failed connections are removed from their respective hub automatically.

- Real-time event broadcast per tenant
- REST endpoint `POST /events` with required `X-Tenant-ID` header
- WebSocket connections identified by tenant ID
- Tenants must never receive events belonging to another tenant

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

## Frontend

Visiting <http://localhost:8080> serves `frontend/index.html`. Each window can
connect as a specific tenant by choosing "Tenant A" or "Tenant B" from the
dropdown and clicking **Connect**.

Open two browser windows and connect each one using a different tenant. Events
posted from one window appear only in windows connected to the same tenant. Use
the text box and **Send** button to publish new events. Newly arrived events are
highlighted briefly so you can easily see updates as they stream in.

## Demo

Start the server and observe the log output:

```bash
$ cd backend
$ go run .
2025/07/31 17:44:45.000000 UTC listening on :8080
2025/07/31 17:44:50.123456 UTC tenant tenantA: websocket connection established
2025/07/31 17:44:53.654321 UTC tenant tenantA: event posted: hello (took 200µs)
```

The frontend lists each event with a local timestamp:

```
17:44:53 - hello
```

## Testing

```
cd backend
go test ./...
```
