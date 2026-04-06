package backend

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
)

type Backend interface {
	SetAlive(bool)
	IsAlive() bool
	GetUR() *url.URL
	GetActiveConnections() int
	Serve(http.ResponseWriter, *http.Request)
}

type backend struct {
	url *url.URL
	alive bool
	mux sync.RWMutex
	connections int
	reverseProxy *httputil.ReverseProxy
}

type ServerPool interface {
	GetBackend() []backend
	GetNextValidPeer() backend
	AddBackend(backend)
	GetServerPoolSize() int
}
