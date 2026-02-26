package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-zen-chu/vofvof/internal/member"
	"github.com/go-zen-chu/vofvof/internal/server"
)

func newTestServer() http.Handler {
	store := member.NewStore()
	return server.New(store)
}

func TestListMembersEmpty(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/members", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var members []map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&members); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(members) != 0 {
		t.Errorf("expected empty list, got %d", len(members))
	}
}

func TestJoinAndListMembers(t *testing.T) {
	srv := newTestServer()

	body, _ := json.Marshal(map[string]string{"name": "Alice", "platform": "macOS"})
	req := httptest.NewRequest(http.MethodPost, "/api/members", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	var m map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m["name"] != "Alice" {
		t.Errorf("expected name Alice, got %v", m["name"])
	}
	if m["status"] != "online" {
		t.Errorf("expected status online, got %v", m["status"])
	}
}

func TestUpdateStatus(t *testing.T) {
	srv := newTestServer()

	// join
	body, _ := json.Marshal(map[string]string{"name": "Bob", "platform": "Windows"})
	req := httptest.NewRequest(http.MethodPost, "/api/members", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var m map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&m); err != nil {
		t.Fatalf("decode join response: %v", err)
	}
	id := m["id"].(string)

	// update status
	statusBody, _ := json.Marshal(map[string]string{"status": "busy"})
	req2 := httptest.NewRequest(http.MethodPut, "/api/members/"+id+"/status", bytes.NewReader(statusBody))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}
	var updated map[string]interface{}
	if err := json.NewDecoder(rec2.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updated["status"] != "busy" {
		t.Errorf("expected status busy, got %v", updated["status"])
	}
}

func TestLeave(t *testing.T) {
	srv := newTestServer()

	// join
	body, _ := json.Marshal(map[string]string{"name": "Carol", "platform": "macOS"})
	req := httptest.NewRequest(http.MethodPost, "/api/members", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var m map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&m); err != nil {
		t.Fatalf("decode join response: %v", err)
	}
	id := m["id"].(string)

	// leave
	req2 := httptest.NewRequest(http.MethodDelete, "/api/members/"+id, nil)
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec2.Code)
	}

	// list should be empty
	req3 := httptest.NewRequest(http.MethodGet, "/api/members", nil)
	rec3 := httptest.NewRecorder()
	srv.ServeHTTP(rec3, req3)
	var members []map[string]interface{}
	if err := json.NewDecoder(rec3.Body).Decode(&members); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(members) != 0 {
		t.Errorf("expected empty list after leave, got %d", len(members))
	}
}

func TestJoinMissingName(t *testing.T) {
	srv := newTestServer()
	body, _ := json.Marshal(map[string]string{"platform": "macOS"})
	req := httptest.NewRequest(http.MethodPost, "/api/members", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

