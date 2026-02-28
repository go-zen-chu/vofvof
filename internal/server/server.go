package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/go-zen-chu/vofvof/internal/member"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// hub manages WebSocket connections and broadcasts member updates.
type hub struct {
	mu    sync.RWMutex
	conns map[*websocket.Conn]struct{}
}

func newHub() *hub {
	return &hub{conns: make(map[*websocket.Conn]struct{})}
}

func (h *hub) add(c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[c] = struct{}{}
}

func (h *hub) remove(c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns, c)
}

func (h *hub) broadcast(msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("broadcast marshal", "err", err)
		return
	}
	h.mu.RLock()
	conns := make([]*websocket.Conn, 0, len(h.conns))
	for c := range h.conns {
		conns = append(conns, c)
	}
	h.mu.RUnlock()
	for _, c := range conns {
		if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
			slog.Warn("ws write error", "err", err)
		}
	}
}

// Server is the virtual office HTTP server.
type Server struct {
	store  *member.Store
	hub    *hub
	router *http.ServeMux
}

// New creates a new Server with all routes registered.
func New(store *member.Store) *Server {
	s := &Server{
		store:  store,
		hub:    newHub(),
		router: http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.router.HandleFunc("/api/members", s.handleMembers)
	s.router.HandleFunc("/api/members/", s.handleMember)
	s.router.HandleFunc("/api/signal", s.handleSignal)
	s.router.HandleFunc("/ws", s.handleWS)
	s.router.HandleFunc("/", s.handleStatic)
}

// wsEvent is the message sent over WebSocket to all clients.
type wsEvent struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// joinRequest is the body for POST /api/members.
type joinRequest struct {
	Name     string `json:"name"`
	Platform string `json:"platform"`
}

// statusRequest is the body for PUT /api/members/:id/status.
type statusRequest struct {
	Status member.Status `json:"status"`
}

// signalRequest is used to relay WebRTC signaling messages between peers.
type signalRequest struct {
	From   string      `json:"from"`
	To     string      `json:"to"`
	Signal interface{} `json:"signal"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writeJSON", "err", err)
	}
}

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// handleMembers handles GET /api/members and POST /api/members.
func (s *Server) handleMembers(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.store.List())
	case http.MethodPost:
		var req joinRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
			return
		}
		m := s.store.Add(req.Name, req.Platform)
		s.hub.broadcast(wsEvent{Type: "member_joined", Payload: m})
		writeJSON(w, http.StatusCreated, m)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleMember handles DELETE /api/members/:id and PUT /api/members/:id/status.
func (s *Server) handleMember(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// path: /api/members/<id>[/status]
	path := strings.TrimPrefix(r.URL.Path, "/api/members/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]

	if len(parts) == 2 && parts[1] == "status" {
		// PUT /api/members/:id/status
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req statusRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}
		if err := s.store.UpdateStatus(id, req.Status); err != nil {
			if errors.Is(err, member.ErrInvalidStatus) {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			} else {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			}
			return
		}
		m := s.store.Get(id)
		if m == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "member not found"})
			return
		}
		s.hub.broadcast(wsEvent{Type: "status_changed", Payload: m})
		writeJSON(w, http.StatusOK, m)
		return
	}

	// DELETE /api/members/:id
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := s.store.Remove(id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	s.hub.broadcast(wsEvent{Type: "member_left", Payload: map[string]string{"id": id}})
	w.WriteHeader(http.StatusNoContent)
}

// handleSignal relays a WebRTC signaling message to all clients.
// In a production setup this would be targeted to the specific peer,
// but for simplicity we broadcast and let the client filter by "to".
func (s *Server) handleSignal(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req signalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	s.hub.broadcast(wsEvent{Type: "signal", Payload: req})
	w.WriteHeader(http.StatusNoContent)
}

// handleWS upgrades the connection to a WebSocket.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("ws upgrade", "err", err)
		return
	}
	s.hub.add(conn)
	defer func() {
		s.hub.remove(conn)
		conn.Close()
	}()
	// Keep the connection alive; client messages are ignored.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

// handleStatic serves static frontend files from the "frontend/dist" directory.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	http.FileServer(http.Dir("frontend/dist")).ServeHTTP(w, r)
}
