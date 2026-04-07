package main

import (
	"net/http/httputil"
	"net/url"
	"sync"
)

// BACKEND REPRESENTS A SINGLE SERVER
type Backend struct {
	URL *url.URL
	Alive bool
	mu sync.RWMutex
	ReverseProxy *httputil.ReverseProxy
	ActiveConnections int64
}

func (b *Backend) IsAlive() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Alive
}

func (b *Backend) SetAlive(alive bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Alive = alive
}

// SERVERPOOL MANAGES ALL OF THE BACKEND
type ServerPool struct {
	backends []*Backend
	mu sync.RWMutex
	current uint64
}

// ADDBACKEND ADDS A NEW BACKEND TO THE POOL
func (s *ServerPool) AddBackend(backend *Backend) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.backends = append(s.backends, backend)
}
