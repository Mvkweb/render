package query

import (
	"math/rand"
	"sync"
	"time"
)

// Manager manages a list of queries and selects them randomly.
type Manager struct {
	queries []string
	mu      sync.Mutex
}

// NewManager creates a new query manager.
func NewManager(queries []string) *Manager {
	return &Manager{
		queries: queries,
	}
}

// GetRandom returns a random query from the list.
func (m *Manager) GetRandom() (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.queries) == 0 {
		return "", false
	}

	rand.Seed(time.Now().UnixNano())
	return m.queries[rand.Intn(len(m.queries))], true
}
