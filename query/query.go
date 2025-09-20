package query

import (
	"sync"
)

// QueryManager manages a list of queries.
type QueryManager struct {
	queries []string
	index   int
	mu      sync.Mutex
}

// NewQueryManager creates a new QueryManager.
func NewQueryManager(queries []string) *QueryManager {
	return &QueryManager{
		queries: queries,
	}
}

// GetNextQuery returns the next query in the list.
func (qm *QueryManager) GetNextQuery() (string, bool) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if qm.index >= len(qm.queries) {
		return "", false // No more queries
	}

	query := qm.queries[qm.index]
	qm.index++
	return query, true
}
