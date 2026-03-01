// Package e2e contains end-to-end tests that spin up a real HTTP server
// (using httptest.NewServer) and exercise the full API including WebSocket
// connections and WebRTC signaling relay.
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-zen-chu/vofvof/internal/member"
	"github.com/go-zen-chu/vofvof/internal/server"
	"github.com/gorilla/websocket"
)

// startServer creates a real HTTP test server and returns its URL.
func startServer(t *testing.T) string {
	t.Helper()
	ts := httptest.NewServer(server.New(member.NewStore()))
	t.Cleanup(ts.Close)
	return ts.URL
}

// post is a helper that sends a POST request with a JSON body and returns the response.
func post(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(data)) //nolint:noctx
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func decodeBody(t *testing.T, r io.Reader, out any) {
	t.Helper()
	if err := json.NewDecoder(r).Decode(out); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

// ─── Full member lifecycle ────────────────────────────────────────────────────

func TestE2E_MemberLifecycle(t *testing.T) {
	base := startServer(t)

	// 1. Initially empty
	resp, err := http.Get(base + "/api/members") //nolint:noctx
	if err != nil {
		t.Fatalf("GET members: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var empty []interface{}
	decodeBody(t, resp.Body, &empty)
	if len(empty) != 0 {
		t.Fatalf("expected empty list, got %d", len(empty))
	}

	// 2. Join
	joinResp := post(t, base+"/api/members", map[string]string{"name": "Alice", "platform": "macOS"})
	defer joinResp.Body.Close()
	if joinResp.StatusCode != http.StatusCreated {
		t.Fatalf("join: expected 201, got %d", joinResp.StatusCode)
	}
	var alice map[string]interface{}
	decodeBody(t, joinResp.Body, &alice)
	aliceID := alice["id"].(string)
	if aliceID == "" {
		t.Fatal("expected non-empty ID")
	}
	if alice["status"] != "online" {
		t.Errorf("expected status online, got %v", alice["status"])
	}

	// 3. List shows Alice
	resp2, err := http.Get(base + "/api/members") //nolint:noctx
	if err != nil {
		t.Fatalf("GET members: %v", err)
	}
	defer resp2.Body.Close()
	var members []map[string]interface{}
	decodeBody(t, resp2.Body, &members)
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	if members[0]["name"] != "Alice" {
		t.Errorf("expected Alice, got %v", members[0]["name"])
	}

	// 4. Update status
	client := &http.Client{}
	statusBody, _ := json.Marshal(map[string]string{"status": "busy"})
	req, _ := http.NewRequest(http.MethodPut, base+"/api/members/"+aliceID+"/status",
		bytes.NewReader(statusBody))
	req.Header.Set("Content-Type", "application/json")
	statusResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("PUT status: %v", err)
	}
	defer statusResp.Body.Close()
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("update status: expected 200, got %d", statusResp.StatusCode)
	}
	var updated map[string]interface{}
	decodeBody(t, statusResp.Body, &updated)
	if updated["status"] != "busy" {
		t.Errorf("expected status busy, got %v", updated["status"])
	}

	// 5. Leave
	delReq, _ := http.NewRequest(http.MethodDelete, base+"/api/members/"+aliceID, nil)
	delResp, err := client.Do(delReq)
	if err != nil {
		t.Fatalf("DELETE member: %v", err)
	}
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("leave: expected 204, got %d", delResp.StatusCode)
	}

	// 6. List is empty again
	resp3, err := http.Get(base + "/api/members") //nolint:noctx
	if err != nil {
		t.Fatalf("GET members: %v", err)
	}
	defer resp3.Body.Close()
	var empty2 []interface{}
	decodeBody(t, resp3.Body, &empty2)
	if len(empty2) != 0 {
		t.Fatalf("expected empty list after leave, got %d", len(empty2))
	}
}

// ─── WebSocket: receives real-time events ────────────────────────────────────

func wsURL(httpURL string) string {
	return strings.Replace(httpURL, "http://", "ws://", 1)
}

func dialWS(t *testing.T, base string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(wsURL(base)+"/ws", nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func readWSEvent(t *testing.T, conn *websocket.Conn, timeout time.Duration) map[string]interface{} {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(timeout)) //nolint:errcheck
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	var event map[string]interface{}
	if err := json.Unmarshal(msg, &event); err != nil {
		t.Fatalf("ws unmarshal: %v", err)
	}
	return event
}

func TestE2E_WebSocket_MemberJoined(t *testing.T) {
	base := startServer(t)
	conn := dialWS(t, base)

	// Join via REST; WebSocket should receive the event.
	joinResp := post(t, base+"/api/members", map[string]string{"name": "Alice", "platform": "macOS"})
	defer joinResp.Body.Close()
	io.Copy(io.Discard, joinResp.Body) //nolint:errcheck

	event := readWSEvent(t, conn, 3*time.Second)
	if event["type"] != "member_joined" {
		t.Errorf("expected event type member_joined, got %v", event["type"])
	}
	payload, ok := event["payload"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected payload to be a map, got %T", event["payload"])
	}
	if payload["name"] != "Alice" {
		t.Errorf("expected name Alice in payload, got %v", payload["name"])
	}
}

func TestE2E_WebSocket_MemberLeft(t *testing.T) {
	base := startServer(t)

	// Join first
	joinResp := post(t, base+"/api/members", map[string]string{"name": "Bob", "platform": "Windows"})
	defer joinResp.Body.Close()
	var bob map[string]interface{}
	decodeBody(t, joinResp.Body, &bob)
	bobID := bob["id"].(string)

	// Connect WS after joining so we only see the leave event
	conn := dialWS(t, base)

	// Leave
	client := &http.Client{}
	req, _ := http.NewRequest(http.MethodDelete, base+"/api/members/"+bobID, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer resp.Body.Close()

	event := readWSEvent(t, conn, 3*time.Second)
	if event["type"] != "member_left" {
		t.Errorf("expected event type member_left, got %v", event["type"])
	}
	payload, ok := event["payload"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected payload map, got %T", event["payload"])
	}
	if payload["id"] != bobID {
		t.Errorf("expected id %s in payload, got %v", bobID, payload["id"])
	}
}

func TestE2E_WebSocket_StatusChanged(t *testing.T) {
	base := startServer(t)

	// Join
	joinResp := post(t, base+"/api/members", map[string]string{"name": "Carol", "platform": "macOS"})
	defer joinResp.Body.Close()
	var carol map[string]interface{}
	decodeBody(t, joinResp.Body, &carol)
	carolID := carol["id"].(string)

	conn := dialWS(t, base)

	// Update status
	statusBody, _ := json.Marshal(map[string]string{"status": "away"})
	req, _ := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/api/members/%s/status", base, carolID),
		bytes.NewReader(statusBody))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("PUT status: %v", err)
	}
	defer resp.Body.Close()

	event := readWSEvent(t, conn, 3*time.Second)
	if event["type"] != "status_changed" {
		t.Errorf("expected status_changed, got %v", event["type"])
	}
	payload, ok := event["payload"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected payload map, got %T", event["payload"])
	}
	if payload["status"] != "away" {
		t.Errorf("expected status away, got %v", payload["status"])
	}
}

// ─── WebRTC signaling relay ───────────────────────────────────────────────────

func TestE2E_SignalRelay(t *testing.T) {
	base := startServer(t)
	conn := dialWS(t, base)

	signalPayload := map[string]interface{}{
		"from":   "peer-a",
		"to":     "peer-b",
		"signal": map[string]string{"type": "offer", "sdp": "v=0..."},
	}
	signalResp := post(t, base+"/api/signal", signalPayload)
	defer signalResp.Body.Close()
	if signalResp.StatusCode != http.StatusNoContent {
		t.Fatalf("signal: expected 204, got %d", signalResp.StatusCode)
	}

	event := readWSEvent(t, conn, 3*time.Second)
	if event["type"] != "signal" {
		t.Errorf("expected signal event, got %v", event["type"])
	}
	payload, ok := event["payload"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected payload map, got %T", event["payload"])
	}
	if payload["from"] != "peer-a" {
		t.Errorf("expected from peer-a, got %v", payload["from"])
	}
	if payload["to"] != "peer-b" {
		t.Errorf("expected to peer-b, got %v", payload["to"])
	}
}

// ─── Multiple members ─────────────────────────────────────────────────────────

func TestE2E_MultipleMembers(t *testing.T) {
	base := startServer(t)
	names := []string{"Alice", "Bob", "Carol", "Dave"}

	for _, name := range names {
		resp := post(t, base+"/api/members", map[string]string{
			"name": name, "platform": "Linux",
		})
		resp.Body.Close()
	}

	resp, err := http.Get(base + "/api/members") //nolint:noctx
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var members []interface{}
	decodeBody(t, resp.Body, &members)
	if len(members) != len(names) {
		t.Errorf("expected %d members, got %d", len(names), len(members))
	}
}
