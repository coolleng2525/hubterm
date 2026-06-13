// Package health provides health check registration and execution.
package health

import (
	"sync"
)

// CheckResult holds the outcome of a single health check.
type CheckResult struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok / degraded / down
	Detail string `json:"detail,omitempty"`
}

// CheckFunc is a health check function.
type CheckFunc func() CheckResult

type registeredCheck struct {
	Name string
	Fn   CheckFunc
}

var (
	checks []registeredCheck
	mu     sync.RWMutex
)

// Register adds a health check.
func Register(name string, fn CheckFunc) {
	mu.Lock()
	checks = append(checks, registeredCheck{Name: name, Fn: fn})
	mu.Unlock()
}

// RunAll executes all registered health checks and returns results.
func RunAll() []CheckResult {
	mu.RLock()
	list := make([]registeredCheck, len(checks))
	copy(list, checks)
	mu.RUnlock()

	results := make([]CheckResult, 0, len(list))
	for _, c := range list {
		results = append(results, c.Fn())
	}
	return results
}
