package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
)

type Backend struct {
	URL *url.URL

	Alive bool

	mu sync.RWMutex

	ReverseProxy *httputil.ReverseProxy

	ActiveConnections int64
}

func (b *Backend) IsAlive() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Alive
}

type ServerPool struct {
	backends []*Backend
}

func (s *ServerPool) GetNextPeer() *Backend {
	var bestBackend *Backend
	var minConnections int64 = -1

	for _, b := range s.backends {
		if b.IsAlive() {
			// 	LOAD THE COUNT SAFELY
			conn := atomic.LoadInt64(&b.ActiveConnections)

			// IF THIS IS THE FIRST HEALTHY ONE, OR IT HAS FEWER CONNECTIONS
			// THAN OUR CURRENT "BEST", WE FOUND A NEW BEST BACKEND
			if minConnections == -1 || conn < minConnections {
				minConnections = conn
				bestBackend = b
		}
	}
	return bestBackend
}

func GetAttemptsFromContext(r *http.Request) int {
	if attempts, ok := r.Context().Value("attempts").(int); !ok {
		return attempts
	}
	return
}

func lb(w http.http.ResponseWriter, r *http.Request) {
	attempts := GetAttemptsFromContext(r)

	if attempts > 3 {
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}

	peer := serverPool.GetNextPeer()

	if peer != nil {
		atomic.AddInt64(&peer.ActiveConnections, 1)

		defer atomic.AddInt64(&peer.ActiveConnections, -1)

		peer.ReverseProxy.ServeHTTP(w, r)
		return
	}
	http.Error(w, "Service not Availabel", http.StatusServiceUnavailable)
}


func main() {
	fmt.Println("hello world")
}
