package member

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ErrInvalidStatus is returned when an unknown status value is provided.
var ErrInvalidStatus = errors.New("invalid status")

// Status represents the online status of a member.
type Status string

const (
	StatusOnline  Status = "online"
	StatusBusy    Status = "busy"
	StatusAway    Status = "away"
	StatusOffline Status = "offline"
)

// Member represents a participant in the virtual office.
type Member struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Platform string    `json:"platform"`
	Status   Status    `json:"status"`
	LastSeen time.Time `json:"lastSeen"`
}

// Store holds all members in memory with concurrent access support.
type Store struct {
	mu      sync.RWMutex
	members map[string]*Member
}

// NewStore creates a new empty member store.
func NewStore() *Store {
	return &Store{
		members: make(map[string]*Member),
	}
}

// Add registers a new member and returns the created member.
func (s *Store) Add(name, platform string) *Member {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := &Member{
		ID:       uuid.New().String(),
		Name:     name,
		Platform: platform,
		Status:   StatusOnline,
		LastSeen: time.Now(),
	}
	s.members[m.ID] = m
	copy := *m
	return &copy
}

// Remove deletes a member by ID. Returns an error if the member does not exist.
func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.members[id]; !ok {
		return fmt.Errorf("member %s not found", id)
	}
	delete(s.members, id)
	return nil
}

// UpdateStatus changes the status of a member. Returns an error if not found or status is invalid.
func (s *Store) UpdateStatus(id string, status Status) error {
	switch status {
	case StatusOnline, StatusBusy, StatusAway, StatusOffline:
	default:
		return fmt.Errorf("%w: %q", ErrInvalidStatus, status)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.members[id]
	if !ok {
		return fmt.Errorf("member %s not found", id)
	}
	m.Status = status
	m.LastSeen = time.Now()
	return nil
}

// List returns a snapshot of all current members.
func (s *Store) List() []*Member {
	s.mu.RLock()
	defer s.mu.RUnlock()
	members := make([]*Member, 0, len(s.members))
	for _, m := range s.members {
		copy := *m
		members = append(members, &copy)
	}
	return members
}

// Get returns a member by ID. Returns nil if not found.
func (s *Store) Get(id string) *Member {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.members[id]
	if !ok {
		return nil
	}
	copy := *m
	return &copy
}
