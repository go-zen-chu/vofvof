package member_test

import (
	"sync"
	"testing"
	"time"

	"github.com/go-zen-chu/vofvof/internal/member"
)

func TestNewStore_Empty(t *testing.T) {
	s := member.NewStore()
	members := s.List()
	if len(members) != 0 {
		t.Errorf("expected empty store, got %d members", len(members))
	}
}

func TestAdd(t *testing.T) {
	s := member.NewStore()
	m := s.Add("Alice", "macOS")

	if m.ID == "" {
		t.Error("expected non-empty ID")
	}
	if m.Name != "Alice" {
		t.Errorf("expected name Alice, got %s", m.Name)
	}
	if m.Platform != "macOS" {
		t.Errorf("expected platform macOS, got %s", m.Platform)
	}
	if m.Status != member.StatusOnline {
		t.Errorf("expected status online, got %s", m.Status)
	}
	if m.LastSeen.IsZero() {
		t.Error("expected LastSeen to be set")
	}
}

func TestAdd_AssignsUniqueIDs(t *testing.T) {
	s := member.NewStore()
	m1 := s.Add("Alice", "macOS")
	m2 := s.Add("Bob", "Windows")
	if m1.ID == m2.ID {
		t.Errorf("expected unique IDs, both got %s", m1.ID)
	}
}

func TestList_ReturnsAllMembers(t *testing.T) {
	s := member.NewStore()
	s.Add("Alice", "macOS")
	s.Add("Bob", "Windows")

	members := s.List()
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
}

func TestList_ReturnsCopies(t *testing.T) {
	s := member.NewStore()
	s.Add("Alice", "macOS")

	list := s.List()
	list[0].Name = "Mutated"

	list2 := s.List()
	if list2[0].Name == "Mutated" {
		t.Error("List() should return copies, not references to internal state")
	}
}

func TestGet_ExistingMember(t *testing.T) {
	s := member.NewStore()
	added := s.Add("Alice", "macOS")

	got := s.Get(added.ID)
	if got == nil {
		t.Fatal("expected member, got nil")
	}
	if got.ID != added.ID {
		t.Errorf("expected ID %s, got %s", added.ID, got.ID)
	}
}

func TestGet_NonExistingMember(t *testing.T) {
	s := member.NewStore()
	got := s.Get("nonexistent")
	if got != nil {
		t.Errorf("expected nil for unknown ID, got %v", got)
	}
}

func TestRemove_ExistingMember(t *testing.T) {
	s := member.NewStore()
	m := s.Add("Alice", "macOS")

	if err := s.Remove(m.ID); err != nil {
		t.Fatalf("unexpected error removing member: %v", err)
	}
	if s.Get(m.ID) != nil {
		t.Error("member should be gone after Remove")
	}
	if len(s.List()) != 0 {
		t.Errorf("expected empty store after remove, got %d", len(s.List()))
	}
}

func TestRemove_NonExistingMember(t *testing.T) {
	s := member.NewStore()
	err := s.Remove("nonexistent")
	if err == nil {
		t.Error("expected error when removing nonexistent member")
	}
}

func TestUpdateStatus_ValidTransitions(t *testing.T) {
	statuses := []member.Status{
		member.StatusBusy,
		member.StatusAway,
		member.StatusOffline,
		member.StatusOnline,
	}
	s := member.NewStore()
	m := s.Add("Alice", "macOS")

	for _, status := range statuses {
		if err := s.UpdateStatus(m.ID, status); err != nil {
			t.Fatalf("unexpected error setting status %s: %v", status, err)
		}
		got := s.Get(m.ID)
		if got.Status != status {
			t.Errorf("expected status %s, got %s", status, got.Status)
		}
	}
}

func TestUpdateStatus_UpdatesLastSeen(t *testing.T) {
	s := member.NewStore()
	m := s.Add("Alice", "macOS")
	before := m.LastSeen

	time.Sleep(time.Millisecond)
	if err := s.UpdateStatus(m.ID, member.StatusBusy); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	after := s.Get(m.ID).LastSeen
	if !after.After(before) {
		t.Error("expected LastSeen to be updated after status change")
	}
}

func TestUpdateStatus_NonExistingMember(t *testing.T) {
	s := member.NewStore()
	err := s.UpdateStatus("nonexistent", member.StatusBusy)
	if err == nil {
		t.Error("expected error when updating nonexistent member")
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	s := member.NewStore()
	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			m := s.Add("concurrent", "Linux")
			_ = s.List()
			_ = s.Get(m.ID)
			_ = s.UpdateStatus(m.ID, member.StatusBusy)
			_ = s.Remove(m.ID)
		}()
	}
	wg.Wait()
}
