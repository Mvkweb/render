package scraper

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// QueryManager manages the generation of diverse and fresh search queries.
type QueryManager struct {
	baseQueries []string
	modifiers   []string
	used        map[string]time.Time
	mu          sync.RWMutex
}

// NewQueryManager creates a new QueryManager.
func NewQueryManager(baseQueries, modifiers []string) *QueryManager {
	return &QueryManager{
		baseQueries: baseQueries,
		modifiers:   modifiers,
		used:        make(map[string]time.Time),
	}
}

// GetNextQuery builds a query that hasn't been used recently.
func (qm *QueryManager) GetNextQuery() string {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	base := qm.baseQueries[rand.Intn(len(qm.baseQueries))]
	modifier := qm.modifiers[rand.Intn(len(qm.modifiers))]

	query := fmt.Sprintf("%s %s", modifier, base)

	// Check if used in last hour
	if lastUsed, exists := qm.used[query]; exists {
		if time.Since(lastUsed) < time.Hour {
			// Try another combination
			return qm.GetNextQuery()
		}
	}

	qm.used[query] = time.Now()
	return query
}
