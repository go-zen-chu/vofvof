package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-zen-chu/vofvof/internal/member"
	"github.com/go-zen-chu/vofvof/internal/server"
)

func newSrv() http.Handler {
	return server.New(member.NewStore())
}

// jsonBody encodes v as JSON and returns a reader over the bytes.
func jsonBody(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return bytes.NewReader(b)
}

// decodeJSON decodes the recorder's body into out.
func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(out); err != nil {
		t.Fatalf("decode JSON response: %v", err)
	}
}

func joinMember(t *testing.T, srv http.Handler, name, platform string) map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/members", jsonBody(t, map[string]string{
		"name": name, "platform": platform,
	}))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("join: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var m map[string]interface{}
	decodeJSON(t, rec, &m)
	return m
}

// ─── GET /api/members ─────────────────────────────────────────────────────────

func TestHandleMembers_GET_Empty(t *testing.T) {
	srv := newSrv()
	req := httptest.NewRequest(http.MethodGet, "/api/members", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var out []interface{}
	decodeJSON(t, rec, &out)
	if len(out) != 0 {
		t.Errorf("expected empty list, got %d", len(out))
	}
}

func TestHandleMembers_GET_AfterJoin(t *testing.T) {
	srv := newSrv()
	joinMember(t, srv, "Alice", "macOS")
	joinMember(t, srv, "Bob", "Windows")

	req := httptest.NewRequest(http.MethodGet, "/api/members", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var out []interface{}
	decodeJSON(t, rec, &out)
	if len(out) != 2 {
		t.Errorf("expected 2 members, got %d", len(out))
	}
}

// ─── POST /api/members ────────────────────────────────────────────────────────

func TestHandleMembers_POST_Success(t *testing.T) {
	srv := newSrv()
	m := joinMember(t, srv, "Alice", "macOS")

	if m["id"] == "" || m["id"] == nil {
		t.Error("expected non-empty id")
	}
	if m["name"] != "Alice" {
		t.Errorf("expected name Alice, got %v", m["name"])
	}
	if m["status"] != "online" {
		t.Errorf("expected status online, got %v", m["status"])
	}
}

func TestHandleMembers_POST_MissingName(t *testing.T) {
	srv := newSrv()
	req := httptest.NewRequest(http.MethodPost, "/api/members",
		jsonBody(t, map[string]string{"platform": "macOS"}))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleMembers_POST_InvalidJSON(t *testing.T) {
	srv := newSrv()
	req := httptest.NewRequest(http.MethodPost, "/api/members",
		bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleMembers_MethodNotAllowed(t *testing.T) {
	srv := newSrv()
	req := httptest.NewRequest(http.MethodPut, "/api/members", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

// ─── PUT /api/members/:id/status ──────────────────────────────────────────────

func TestHandleMember_UpdateStatus_Success(t *testing.T) {
	srv := newSrv()
	m := joinMember(t, srv, "Bob", "Windows")
	id := m["id"].(string)

	req := httptest.NewRequest(http.MethodPut, "/api/members/"+id+"/status",
		jsonBody(t, map[string]string{"status": "busy"}))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var updated map[string]interface{}
	decodeJSON(t, rec, &updated)
	if updated["status"] != "busy" {
		t.Errorf("expected status busy, got %v", updated["status"])
	}
}

func TestHandleMember_UpdateStatus_NotFound(t *testing.T) {
	srv := newSrv()
	req := httptest.NewRequest(http.MethodPut, "/api/members/unknown/status",
		jsonBody(t, map[string]string{"status": "busy"}))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleMember_UpdateStatus_InvalidJSON(t *testing.T) {
	srv := newSrv()
	m := joinMember(t, srv, "Bob", "Windows")
	id := m["id"].(string)

	req := httptest.NewRequest(http.MethodPut, "/api/members/"+id+"/status",
		bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// ─── DELETE /api/members/:id ──────────────────────────────────────────────────

func TestHandleMember_Delete_Success(t *testing.T) {
	srv := newSrv()
	m := joinMember(t, srv, "Carol", "macOS")
	id := m["id"].(string)

	req := httptest.NewRequest(http.MethodDelete, "/api/members/"+id, nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	// Verify member gone from list
	req2 := httptest.NewRequest(http.MethodGet, "/api/members", nil)
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, req2)
	var members []interface{}
	decodeJSON(t, rec2, &members)
	if len(members) != 0 {
		t.Errorf("expected empty list after delete, got %d", len(members))
	}
}

func TestHandleMember_Delete_NotFound(t *testing.T) {
	srv := newSrv()
	req := httptest.NewRequest(http.MethodDelete, "/api/members/unknown", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleMember_MethodNotAllowed(t *testing.T) {
	srv := newSrv()
	m := joinMember(t, srv, "Dave", "Linux")
	id := m["id"].(string)

	req := httptest.NewRequest(http.MethodPost, "/api/members/"+id, nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

// ─── POST /api/signal ─────────────────────────────────────────────────────────

func TestHandleSignal_Success(t *testing.T) {
	srv := newSrv()
	body := jsonBody(t, map[string]interface{}{
		"from":   "id-a",
		"to":     "id-b",
		"signal": map[string]string{"type": "offer"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/signal", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestHandleSignal_InvalidJSON(t *testing.T) {
	srv := newSrv()
	req := httptest.NewRequest(http.MethodPost, "/api/signal",
		bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleSignal_MethodNotAllowed(t *testing.T) {
	srv := newSrv()
	req := httptest.NewRequest(http.MethodGet, "/api/signal", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

// ─── CORS preflight ───────────────────────────────────────────────────────────

func TestCORSPreflight_Members(t *testing.T) {
	srv := newSrv()
	req := httptest.NewRequest(http.MethodOptions, "/api/members", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected CORS header, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}
