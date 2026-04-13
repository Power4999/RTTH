package persist

import (
	"RTTH/internal/structs"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type State struct {
	CurrentTerm int                   `json:"current_term"`
	VotedFor    map[int]int           `json:"voted_for"`
	Log         []structs.Transaction `json:"log"`
	Blockchain  []structs.Block       `json:"blockchain"`
	BlockBuffer []structs.Transaction `json:"block_buffer"`
}

type Storage struct {
	path string
}

// NewStorage performs per-node storage initialization and returns a storage handle.
func NewStorage(dataDir string, nodeID int) (*Storage, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("persist: cannot create data dir %s: %w", dataDir, err)
	}
	return &Storage{
		path: filepath.Join(dataDir, fmt.Sprintf("node_%d_state.json", nodeID)),
	}, nil
}

// Save performs atomic state persistence and returns an error when writing fails.
func (s *Storage) Save(state State) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("persist: marshal failed: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("persist: write temp file failed: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("persist: rename failed: %w", err)
	}
	return nil
}

// Load performs state restoration from disk and returns the recovered state.
func (s *Storage) Load() (State, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return State{
			VotedFor:    map[int]int{},
			Log:         []structs.Transaction{},
			Blockchain:  []structs.Block{},
			BlockBuffer: []structs.Transaction{},
		}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("persist: read failed: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("persist: unmarshal failed: %w", err)
	}

	if state.VotedFor == nil {
		state.VotedFor = map[int]int{}
	}
	if state.Log == nil {
		state.Log = []structs.Transaction{}
	}
	if state.Blockchain == nil {
		state.Blockchain = []structs.Block{}
	}
	if state.BlockBuffer == nil {
		state.BlockBuffer = []structs.Transaction{}
	}
	return state, nil
}
