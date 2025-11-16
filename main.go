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