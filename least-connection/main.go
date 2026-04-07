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

