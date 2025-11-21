package main

import (
	"context"
	"flag"
	"log"
  "net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
  "fmt"
)

// Constants for tracking retries and attempts in request context
const (
	Attempts int = iota // Total attempts for a request across different backends
	Retry               // Retry count for the same backend
)

// GetAttemptsFromContext extracts the retry count from the request's context
// Returns 0 if the value is not set
func GetAttemptsFromContext(r *http.Request) int {
	if retry, ok := r.Context().Value(Retry).(int); ok {
		return retry
	}
	return 0
}

// Backend represents a single backend server
type Backend struct {
	URL          *url.URL                 // Backend URL
	Alive        bool                     // Is the backend alive?
	mux          sync.RWMutex             // Protects concurrent access to Alive
	ReverseProxy *httputil.ReverseProxy  // Reverse proxy to forward requests
}

// ServerPool tracks all backends and the current index for round-robin
type ServerPool struct {
	backends []*Backend
	current  uint64 // atomic counter for round-robin
}

// SetAlive updates the backend's alive status
func (b *Backend) SetAlive(alive bool) {
	b.mux.Lock()
	b.Alive = alive
	b.mux.Unlock()
}

// IsAlive safely returns whether the backend is alive
func (b *Backend) IsAlive() bool {
	b.mux.RLock()
	defer b.mux.RUnlock()
	return b.Alive
}

// NextIndex returns the next backend index in a round-robin fashion
// Atomic increment ensures concurrent requests do not pick the same backend
func (s *ServerPool) NextIndex() int {
	return int(atomic.AddUint64(&s.current, 1) % uint64(len(s.backends)))
}

// GetNextPeer returns the next alive backend in round-robin order
// Updates the current index if it chooses a different backend than expected
func (s *ServerPool) GetNextPeer() *Backend {
	nextIdx := s.NextIndex()
	l := len(s.backends) + nextIdx // loop through all backends once
	for i := nextIdx; i < l; i++ {
		idx := i % len(s.backends)
		if s.backends[idx].IsAlive() {
			if i != nextIdx {
				atomic.StoreUint64(&s.current, uint64(idx))
			}
			return s.backends[idx]
		}
	}
	return nil
}

// MarkBackendStatus sets a backend's alive/dead status by URL
func (s *ServerPool) MarkBackendStatus(backendUrl *url.URL, alive bool) {
	for _, b := range s.backends {
		if b.URL.String() == backendUrl.String() {
			b.SetAlive(alive)
			break
		}
	}
}

// lb is the HTTP handler for the load balancer
// It forwards requests to available backends or returns 503 if none are alive
func lb(w http.ResponseWriter, r *http.Request) {
	attempts := GetAttemptsFromContext(r)

	if attempts > 3 {
		// Too many attempts, give up
		log.Printf("%s(%s) Max attempts reached, terminating\n", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}

	peer := serverPool.GetNextPeer()
	if peer != nil {
		// Forward request to chosen backend
		peer.ReverseProxy.ServeHTTP(w, r)
		return
	}

	// No backends available
	http.Error(w, "Service not available", http.StatusServiceUnavailable)
}

// isBackendAlive checks whether a backend server is reachable.
// It does this by attempting to establish a TCP connection to the backend's host and port.
// Returns true if the connection succeeds, false otherwise.
func isBackendAlive(u *url.URL) bool {
	timeout := 2 * time.Second // Set a 2-second timeout for the connection attempt

	// Attempt a TCP connection to the backend
	conn, err := net.DialTimeout("tcp", u.Host, timeout)
	if err != nil {
		// Connection failed, log the error and return false
		log.Println("Site unreachable, error: ", err)
		return false
	}

	// Connection succeeded, close immediately as we just wanted to check reachability
	_ = conn.Close()
	return true
}


var serverPool ServerPool

func main() {
	// Command-line flags for backends and port
	var serverList string
	var port int
	flag.StringVar(&serverList, "backends", "", "Comma-separated list of backends")
	flag.IntVar(&port, "port", 8080, "Port to serve")
	flag.Parse()

	// Require at least one backend
	if len(serverList) == 0 {
		log.Fatal("Please provide a minimum of one backend server.")
	}

	// Split backends and create ReverseProxy for each
	tokens := strings.Split(serverList, ",")
	for _, tok := range tokens {
		serverUrl, err := url.Parse(tok)
		if err != nil {
			log.Fatal(err) // Exit if URL is invalid
		}

		// Create reverse proxy for backend
		proxy := httputil.NewSingleHostReverseProxy(serverUrl)

		// Set error handler to retry requests or mark backend as down
		proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, e error) {
			log.Printf("[%s] %s\n", serverUrl.Host, e.Error())

			// Retry the same backend up to 3 times
			retries := GetAttemptsFromContext(request)
			if retries < 3 {
				time.Sleep(10 * time.Millisecond)
				ctx := context.WithValue(request.Context(), Retry, retries+1)
				proxy.ServeHTTP(writer, request.WithContext(ctx))
				return
			}

			// Mark backend as dead after retries
			serverPool.MarkBackendStatus(serverUrl, false)

			// Retry request on next available backend
			attempts := GetAttemptsFromContext(request)
			log.Printf("%s(%s) Attempting retry %d\n", request.RemoteAddr, request.URL.Path, attempts)
			ctx := context.WithValue(request.Context(), Attempts, attempts+1)
			lb(writer, request.WithContext(ctx))
		}

		// Add backend to pool
		serverPool.backends = append(serverPool.backends, &Backend{
			URL:          serverUrl,
			Alive:        true,
			ReverseProxy: proxy,
		})

		log.Printf("Added backend: %s\n", serverUrl)
	}

	// Start the HTTP load balancer server
	http.HandleFunc("/", lb)
	log.Printf("Load balancer started on port %d\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
