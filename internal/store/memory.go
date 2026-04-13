package store

import (
	"RTTH/internal/structs"
	"errors"
	"maps"
	"sync"
)

type LogStore interface {
	Append(t structs.Transaction) error
	GetByID(id int) (structs.Transaction, error)
	GetAll() map[int]structs.Transaction
}

type MemoryStore struct {
	Mu        sync.RWMutex
	data      map[int]structs.Transaction
	nextIndex int
}

// NewMemoryStore performs in-memory log store initialization and returns a ready-to-use store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data:      make(map[int]structs.Transaction),
		nextIndex: 1,
	}
}

// Append performs transaction insertion and returns an error when insertion fails.
func (m *MemoryStore) Append(t structs.Transaction) error {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	t.ID = m.nextIndex
	m.nextIndex++
	m.data[t.ID] = t
	return nil
}

// GetByID performs lookup by index and returns the matching transaction.
func (m *MemoryStore) GetByID(id int) (structs.Transaction, error) {
	m.Mu.RLock()
	defer m.Mu.RUnlock()
	t, ok := m.data[id]
	if !ok {
		return structs.Transaction{}, errors.New("transaction not found")
	}
	return t, nil
}

// GetAll performs a snapshot copy of all transactions and returns the copied map.
func (m *MemoryStore) GetAll() map[int]structs.Transaction {
	m.Mu.RLock()
	defer m.Mu.RUnlock()
	temp := make(map[int]structs.Transaction, len(m.data))
	maps.Copy(temp, m.data)
	return temp
}
