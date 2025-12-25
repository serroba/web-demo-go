package store

import (
	"context"
	"sync"

	"github.com/serroba/web-demo-go/internal/handlers"
)

// MemoryStore is an in-memory implementation of URLRepository.
type MemoryStore struct {
	mu     sync.RWMutex
	urls   map[string]string // code -> url
	hashes map[string]string // urlHash -> code
}

// NewMemoryStore creates a new in-memory URL store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		urls:   make(map[string]string),
		hashes: make(map[string]string),
	}
}

func (m *MemoryStore) Save(_ context.Context, code, url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.urls[code] = url

	return nil
}

func (m *MemoryStore) Get(_ context.Context, code string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	url, ok := m.urls[code]
	if !ok {
		return "", handlers.ErrNotFound
	}

	return url, nil
}

func (m *MemoryStore) SaveWithHash(_ context.Context, code, url, urlHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.urls[code] = url
	m.hashes[urlHash] = code

	return nil
}

func (m *MemoryStore) GetCodeByHash(_ context.Context, urlHash string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	code, ok := m.hashes[urlHash]
	if !ok {
		return "", handlers.ErrNotFound
	}

	return code, nil
}
