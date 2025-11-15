package main

import (
	"fmt"
	"net/http/httputil"
	"net/url"
	"sync"
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

func main() {
	fmt.Println("Hello World!")
}