package main

import (
	"fmt"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
)

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

func main() {
	fmt.Println("Hello World!")
}