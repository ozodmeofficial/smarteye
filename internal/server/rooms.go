package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// roomStore persists rooms and device→room assignments to a JSON file so the
// layout survives restarts.
type roomStore struct {
	path string
	mu   sync.Mutex
}

type roomFile struct {
	Rooms  []Room            `json:"rooms"`
	Assign map[string]string `json:"assign"` // deviceID -> roomID
}

func newRoomStore(dir string) *roomStore {
	return &roomStore{path: filepath.Join(dir, "rooms.json")}
}

// Load reads persisted rooms and assignments; missing file yields empty maps.
func (s *roomStore) Load() ([]Room, map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, map[string]string{}
	}
	var rf roomFile
	if json.Unmarshal(data, &rf) != nil {
		return nil, map[string]string{}
	}
	if rf.Assign == nil {
		rf.Assign = map[string]string{}
	}
	return rf.Rooms, rf.Assign
}

// Save atomically writes the current rooms and assignments.
func (s *roomStore) Save(rooms []Room, assign map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rf := roomFile{Rooms: rooms, Assign: assign}
	data, err := json.MarshalIndent(rf, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// persist snapshots the hub's current rooms/assignments to disk.
func (h *Hub) persist() {
	if h.store == nil {
		return
	}
	h.mu.RLock()
	rooms := make([]Room, 0, len(h.rooms))
	for _, r := range h.rooms {
		rooms = append(rooms, *r)
	}
	assign := make(map[string]string, len(h.roomOf))
	for k, v := range h.roomOf {
		assign[k] = v
	}
	h.mu.RUnlock()
	_ = h.store.Save(rooms, assign)
}
