package store_test

import (
	"RTTH/internal/store"
	"RTTH/internal/structs"
	"sync"
	"testing"
)

func TestMemoryStoreBehavior_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, s *store.MemoryStore)
	}{
		{
			name: "append and get by id",
			run: func(t *testing.T, s *store.MemoryStore) {
				txn := structs.Transaction{ClientID: 99, Payload: "A->B 10", Timestamp: 123456789}
				if err := s.Append(txn); err != nil {
					t.Fatalf("Append: unexpected error: %v", err)
				}
				got, err := s.GetByID(1)
				if err != nil {
					t.Fatalf("GetByID(1): unexpected error: %v", err)
				}
				if got.ID != 1 {
					t.Fatalf("expected assigned ID=1, got %d", got.ID)
				}
				if got.ClientID != 99 || got.Payload != "A->B 10" {
					t.Fatalf("unexpected payload: %+v", got)
				}
			},
		},
		{
			name: "get all returns all indexes",
			run: func(t *testing.T, s *store.MemoryStore) {
				_ = s.Append(structs.Transaction{ClientID: 1, Payload: "A->B 10"})
				_ = s.Append(structs.Transaction{ClientID: 2, Payload: "C->D 20"})
				all := s.GetAll()
				if len(all) != 2 {
					t.Fatalf("expected 2 entries, got %d", len(all))
				}
				if _, ok := all[1]; !ok {
					t.Fatalf("expected entry at index 1")
				}
				if _, ok := all[2]; !ok {
					t.Fatalf("expected entry at index 2")
				}
			},
		},
		{
			name: "get by id not found",
			run: func(t *testing.T, s *store.MemoryStore) {
				if _, err := s.GetByID(999); err == nil {
					t.Fatalf("expected error for missing key, got nil")
				}
			},
		},
		{
			name: "sequential indexing overrides caller id",
			run: func(t *testing.T, s *store.MemoryStore) {
				for i := 0; i < 5; i++ {
					_ = s.Append(structs.Transaction{ID: 999, ClientID: i + 1, Payload: "x"})
				}
				all := s.GetAll()
				for idx := 1; idx <= 5; idx++ {
					entry, ok := all[idx]
					if !ok {
						t.Fatalf("expected entry at index %d", idx)
					}
					if entry.ID != idx {
						t.Fatalf("entry at key %d has ID=%d", idx, entry.ID)
					}
				}
			},
		},
		{
			name: "get all returns copy",
			run: func(t *testing.T, s *store.MemoryStore) {
				_ = s.Append(structs.Transaction{ClientID: 1, Payload: "original"})
				copy1 := s.GetAll()
				copy1[1] = structs.Transaction{ClientID: 1, Payload: "mutated"}
				copy2 := s.GetAll()
				if copy2[1].Payload != "original" {
					t.Fatalf("store internal state was mutated through returned map: got %q", copy2[1].Payload)
				}
			},
		},
		{
			name: "first index is one",
			run: func(t *testing.T, s *store.MemoryStore) {
				_ = s.Append(structs.Transaction{ClientID: 5, Payload: "first"})
				got, err := s.GetByID(1)
				if err != nil {
					t.Fatalf("expected entry at ID=1, got error: %v", err)
				}
				if got.ClientID != 5 {
					t.Fatalf("expected ClientID=5, got %d", got.ClientID)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			s := store.NewMemoryStore()
			tt.run(t, s)
		})
	}
}

func TestMemoryStoreConcurrency_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		writers      int
		readers      int
		expectedSize int
	}{
		{
			name:         "parallel readers and writers",
			writers:      100,
			readers:      50,
			expectedSize: 100,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			s := store.NewMemoryStore()
			var wg sync.WaitGroup

			for i := 1; i <= tt.writers; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					_ = s.Append(structs.Transaction{ClientID: id, Payload: "payload", Timestamp: 123})
				}(i)
			}

			for i := 0; i < tt.readers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_ = s.GetAll()
				}()
			}

			wg.Wait()
			if got := len(s.GetAll()); got != tt.expectedSize {
				t.Fatalf("expected %d entries after concurrent writes, got %d", tt.expectedSize, got)
			}
		})
	}
}
