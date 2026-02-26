# vofvof – Virtual Office

A virtual office application with a **Go** backend and a **TypeScript/Vite** frontend.

Team member status is tracked centrally via the backend REST API and broadcast in real-time over WebSockets. Actual audio/video communication between members uses **WebRTC peer-to-peer** connections, with the server acting only as a signaling relay.

## Architecture

```
macOS / Windows browser
  └─ frontend (Vite + TypeScript)
        ├─ REST  ──►  Go backend  (status, join, leave)
        ├─ WebSocket ──► Go backend  (real-time status updates & WebRTC signaling)
        └─ WebRTC P2P ──► other member's browser  (audio + video)
```

## Backend API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/members` | List all members |
| `POST` | `/api/members` | Join the office |
| `DELETE` | `/api/members/:id` | Leave the office |
| `PUT` | `/api/members/:id/status` | Update status (`online`/`busy`/`away`/`offline`) |
| `POST` | `/api/signal` | Relay a WebRTC signaling message |
| `GET` | `/ws` | WebSocket for real-time events |

## Getting Started

### Backend

```sh
go run .
# Starts on :8080 by default.  Override with -addr :9000
```

### Frontend (development)

```sh
cd frontend
npm install
npm run dev      # Vite dev server on :3000, proxies /api and /ws to :8080
```

### Frontend (production build)

```sh
cd frontend
npm run build    # Output goes to frontend/dist/
# The Go server serves frontend/dist/ at /
```

