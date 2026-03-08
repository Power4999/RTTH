package test

import (
	"RTTH/internal/store"
	"RTTH/internal/structs"
	"sync"
	"testing"
)

func TestMemoryStore_AppendAndGetByID(t *testing.T) {
	s := store.NewMemoryStore()
	txn := structs.Transaction{ID: 1, Payload: "A->B 10", Timestamp: 123456789}

	err := s.Append(txn)
	if err != nil {
		t.Fatalf("expected no error on append, got %v", err)
	}

	retrievedTxn, err := s.GetByID(1)
	if err != nil {
		t.Fatalf("expected no error on get, got %v", err)
	}

	if retrievedTxn.ID != txn.ID || retrievedTxn.Payload != txn.Payload {
		t.Errorf("expected %+v, got %+v", txn, retrievedTxn)
	}
}

func TestMemoryStore_GetAll(t *testing.T) {
	s := store.NewMemoryStore()
	s.Append(structs.Transaction{ID: 1, Payload: "A->B 10"})
	s.Append(structs.Transaction{ID: 2, Payload: "C->D 20"})

	all := s.GetAll()
	if len(all) != 2 {
		t.Errorf("expected map of length 2, got %d", len(all))
	}
}

func TestMemoryStore_Concurrency(t *testing.T) {
	s := store.NewMemoryStore()
	var wg sync.WaitGroup

	// Test 100 concurrent writes
	for i := 1; i <= 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			s.Append(structs.Transaction{ID: id, Payload: "test-payload", Timestamp: 123})
		}(i)
	}

	// Test 50 concurrent reads occurring at the same time
	for i := 1; i <= 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.GetAll()
		}()
	}

	wg.Wait()
	if len(s.GetAll()) != 100 {
		t.Errorf("expected 100 items in store, got %d", len(s.GetAll()))
	}
}
