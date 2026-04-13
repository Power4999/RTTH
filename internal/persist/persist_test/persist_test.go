package persist_test

import (
	"RTTH/internal/persist"
	"RTTH/internal/structs"
	"os"
	"path/filepath"
	"testing"
)

func TestStorageLifecycle_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "new storage creates directory",
			run: func(t *testing.T) {
				dir := filepath.Join(t.TempDir(), "nested", "data")
				if _, err := persist.NewStorage(dir, 1); err != nil {
					t.Fatalf("NewStorage: unexpected error: %v", err)
				}
				if _, err := os.Stat(dir); os.IsNotExist(err) {
					t.Fatalf("expected data directory to be created")
				}
			},
		},
		{
			name: "load first boot returns safe defaults",
			run: func(t *testing.T) {
				s, _ := persist.NewStorage(t.TempDir(), 1)
				state, err := s.Load()
				if err != nil {
					t.Fatalf("Load on first boot: unexpected error: %v", err)
				}
				if state.CurrentTerm != 0 {
					t.Fatalf("expected CurrentTerm=0 on first boot, got %d", state.CurrentTerm)
				}
				if state.VotedFor == nil || state.Log == nil || state.Blockchain == nil || state.BlockBuffer == nil {
					t.Fatalf("first boot collections must not be nil")
				}
			},
		},
		{
			name: "save and load round trip",
			run: func(t *testing.T) {
				s, _ := persist.NewStorage(t.TempDir(), 2)
				original := persist.State{
					CurrentTerm: 7,
					VotedFor:    map[int]int{1: 3, 2: 3, 7: 2},
					Log: []structs.Transaction{
						{ID: 1, ClientID: 10, Payload: "A->B 100", Timestamp: 111, Term: 2},
						{ID: 2, ClientID: 11, Payload: "C->D 200", Timestamp: 222, Term: 3},
					},
				}
				if err := s.Save(original); err != nil {
					t.Fatalf("Save: unexpected error: %v", err)
				}
				loaded, err := s.Load()
				if err != nil {
					t.Fatalf("Load: unexpected error: %v", err)
				}
				if loaded.CurrentTerm != original.CurrentTerm {
					t.Fatalf("CurrentTerm: want %d, got %d", original.CurrentTerm, loaded.CurrentTerm)
				}
				if len(loaded.Log) != len(original.Log) {
					t.Fatalf("Log length: want %d, got %d", len(original.Log), len(loaded.Log))
				}
			},
		},
		{
			name: "latest save overwrites old save",
			run: func(t *testing.T) {
				s, _ := persist.NewStorage(t.TempDir(), 1)
				_ = s.Save(persist.State{CurrentTerm: 1, VotedFor: map[int]int{1: 2}})
				_ = s.Save(persist.State{CurrentTerm: 5, VotedFor: map[int]int{5: 3}})
				state, err := s.Load()
				if err != nil {
					t.Fatalf("Load: %v", err)
				}
				if state.CurrentTerm != 5 {
					t.Fatalf("expected latest CurrentTerm=5, got %d", state.CurrentTerm)
				}
				if state.VotedFor[5] != 3 {
					t.Fatalf("expected VotedFor[5]=3, got %d", state.VotedFor[5])
				}
			},
		},
		{
			name: "atomic write leaves no temp file",
			run: func(t *testing.T) {
				dir := t.TempDir()
				s, _ := persist.NewStorage(dir, 1)
				if err := s.Save(persist.State{CurrentTerm: 3, VotedFor: map[int]int{3: 1}}); err != nil {
					t.Fatalf("Save: %v", err)
				}
				tmpPath := filepath.Join(dir, "node_1_state.json.tmp")
				if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
					t.Fatalf("temp file still exists after Save")
				}
				realPath := filepath.Join(dir, "node_1_state.json")
				if _, err := os.Stat(realPath); os.IsNotExist(err) {
					t.Fatalf("state file does not exist after Save")
				}
			},
		},
		{
			name: "per node files stay isolated",
			run: func(t *testing.T) {
				dir := t.TempDir()
				s1, _ := persist.NewStorage(dir, 1)
				s2, _ := persist.NewStorage(dir, 2)
				_ = s1.Save(persist.State{CurrentTerm: 10, VotedFor: map[int]int{}})
				_ = s2.Save(persist.State{CurrentTerm: 20, VotedFor: map[int]int{}})
				loaded1, _ := s1.Load()
				loaded2, _ := s2.Load()
				if loaded1.CurrentTerm != 10 {
					t.Fatalf("node 1: expected term 10, got %d", loaded1.CurrentTerm)
				}
				if loaded2.CurrentTerm != 20 {
					t.Fatalf("node 2: expected term 20, got %d", loaded2.CurrentTerm)
				}
			},
		},
		{
			name: "load initializes omitted collections",
			run: func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "node_1_state.json")
				_ = os.WriteFile(path, []byte(`{"current_term":3}`), 0o644)
				s, _ := persist.NewStorage(dir, 1)
				state, err := s.Load()
				if err != nil {
					t.Fatalf("Load: %v", err)
				}
				if state.VotedFor == nil || state.Log == nil || state.Blockchain == nil || state.BlockBuffer == nil {
					t.Fatalf("loaded collections must not be nil")
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}
