package reliability

import (
	"fmt"
	"sync"
	"time"
)

// CircuitBreaker is a state machine to prevent repeated execution of a failing function.
type CircuitBreaker struct {
	failures     int
	maxFailures  int
	state        string // "closed", "open", "half-open"
	lastFailTime time.Time
	timeout      time.Duration
	mu           sync.RWMutex
}

// NewCircuitBreaker creates a new CircuitBreaker.
func NewCircuitBreaker(maxFailures int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures: maxFailures,
		timeout:     timeout,
		state:       "closed",
	}
}

// Call executes the given function, applying the circuit breaker logic.
func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Check if circuit is open
	if cb.state == "open" {
		if time.Since(cb.lastFailTime) > cb.timeout {
			cb.state = "half-open"
			cb.failures = 0
		} else {
			return fmt.Errorf("circuit breaker is open")
		}
	}

	err := fn()
	if err != nil {
		cb.failures++
		cb.lastFailTime = time.Now()
		if cb.failures >= cb.maxFailures {
			cb.state = "open"
		}
		return err
	}

	// Success - reset
	cb.failures = 0
	cb.state = "closed"
	return nil
}
