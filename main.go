package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
)

// Constants for retry handling in HTTP requests.
// `Attempts` represents the starting index for counting attempts.
// `Retry` is used as a key for storing/retrieving retry count in request context.
// iota implements int values increasing incrementally, much like std::iota in C++.
const (
    Attempts int = iota
    Retry
)

// GetRetryFromContext extracts the retry count from the request context.
// Returns 0 if the retry value is not set.
func GetRetryFromContext(r *http.Request) int {
    if retry, ok := r.Context().Value(Retry).(int); ok {
        return retry
    }
    return 0
}

// Data structure to represent a backend server.
type Backend struct {
	URL *url.URL
	Alive bool
	mux sync.RWMutex
	ReverseProxy *httputil.ReverseProxy
}

// Data structure to track all backends.
type ServerPool struct {
	backends []*Backend
	current uint64
}

// Returns the next index in the backends slice to route the request to. 
// modulus trick is used to keep index in the range [0, len(backends) - 1]
// Imagine multiple requests are happening at the same time (common in a server).
// Without atomic, two requests could run NextIndex() at the exact same time.
// Both read the same current value before either increments it.
// Both could return the same index, even though they should pick different servers.
// This would break the round-robin behavior.
// atomic.AddUint64 guarantees that the increment happens safely, even if many requests happen at the same time.
// No two requests will ever get the same increment value.
// Each request sees a unique current value.
// The round-robin works correctly under concurrent access.
func (s *ServerPool) NextIndex() int {
  return int(atomic.AddUint64(&s.current, uint64(1)) % uint64(len(s.backends)))
}

// Sets backend to be alive.
func (b *Backend) SetAlive(alive bool) {
	b.mux.Lock()
	b.Alive = alive
	b.mux.Unlock()
}

// Helper function to check whether a backend is alive or not.
func (b *Backend) IsAlive() (alive bool) {
	b.mux.RLock()
	alive = b.Alive
	b.mux.RUnlock()
	return
}

// GetNextPeer returns the next available backend server using a round-robin strategy.
// It starts from the next server in line and loops through all backends once to find an alive server.
// If a different server than the original candidate is selected, it updates the current index atomically
// to ensure thread-safe round-robin behavior under concurrent requests.
// Returns nil if no backend is alive.
func (s *ServerPool) GetNextPeer() *Backend {
  next_idx := s.NextIndex()
  l := len(s.backends) + next_idx // start from next_idx and move a full cycle
  for i := next_idx; i < l; i++ {
    idx := i % len(s.backends) 
    if s.backends[idx].IsAlive() {
      if i != next_idx {
        atomic.StoreUint64(&s.current, uint64(idx)) 
      }
      return s.backends[idx]
    }
  }
  return nil
}

// lb is the main load balancer handler for incoming HTTP requests.
// It selects the next available backend using GetNextPeer().
// If an alive backend is found, the request is forwarded to its ReverseProxy.
// If no backends are available, it responds with a 503 Service Unavailable error.
func lb(w http.ResponseWriter, r *http.Request) {
	peer := serverPool.GetNextPeer()
	if peer != nil {
		peer.ReverseProxy.ServeHTTP(w, r)
		return 
	} 
	http.Error(w, "Service not available", http.StatusServiceUnavailable)
}

var serverPool ServerPool
 
func main() {
	fmt.Println("Hello World!")
}