# agents.md

## 📌 Project: Real-Time Multi-Tenant Event Feed

### 🧠 Objective
Build a backend system and simple frontend that allows multiple tenants to:
- Connect over WebSocket
- Send/receive events in real-time
- Maintain strict tenant-level isolation

---

## ⚙️ Tech Stack

- **Backend**: Go (net/http, Gorilla WebSocket)
- **Frontend**: HTML + Vanilla JS WebSocket client
- **Storage**: In-memory (per-tenant)
- **Auth**: Simplified – tenant ID passed via header or query param

---

## 🧱 Architecture Overview

### Components

1. **REST API**
   - `POST /events` – Accepts new event for a tenant
   - Header: `X-Tenant-ID: tenant_a`
   - Stores event in memory
   - Notifies relevant WebSocket clients

2. **WebSocket Server**
   - Endpoint: `/ws?tenant_id=...`
   - Registers client under the correct tenant
   - Receives broadcast events for that tenant only

3. **Event Broker**
   - Manages:
     - Per-tenant event queues
     - Active WebSocket connections (map of tenant → set of connections)
   - Dispatches events to connections via channels

4. **Frontend**
   - Dropdown to select tenant (A or B)
   - WebSocket connection to backend
   - Listens for real-time events
   - Form to send new event
   - Highlights received messages clearly

---

## 📦 Data Models

```go
type Event struct {
    ID        string    `json:"id"`
    TenantID  string    `json:"tenant_id"`
    Message   string    `json:"message"`
    Timestamp time.Time `json:"timestamp"`
}

```
