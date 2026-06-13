// Package repair provides self-repair infrastructure with retry and backoff.
package repair

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// Action is a repair action.
type Action struct {
	Name string
	Run  func() error
}

var (
	repairMap   = make(map[string][]Action)
	repairMapMu sync.RWMutex
)

// OnFailure registers a repair action for a given check name.
func OnFailure(checkName string, action Action) {
	repairMapMu.Lock()
	repairMap[checkName] = append(repairMap[checkName], action)
	repairMapMu.Unlock()
}

// Attempt executes a repair action with retry and exponential backoff.
func Attempt(action Action, maxRetries int) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		log.Printf("[repair] attempting %q (attempt %d/%d)", action.Name, i+1, maxRetries)
		err := action.Run()
		if err == nil {
			log.Printf("[repair] %q succeeded", action.Name)
			return nil
		}
		lastErr = err
		log.Printf("[repair] %q failed: %v", action.Name, err)
		if i < maxRetries-1 {
			backoff := time.Duration(1<<uint(i)) * time.Second
			time.Sleep(backoff)
		}
	}
	return fmt.Errorf("%q failed after %d retries: %w", action.Name, maxRetries, lastErr)
}
