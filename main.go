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

func main() {
	fmt.Println("Hello World!")
}