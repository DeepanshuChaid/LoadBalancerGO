package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// Backend represents a single backend server
type Backend struct {
	URL              *url.URL
	Alive            bool
	mu               sync.RWMutex
	ReverseProxy     *httputil.ReverseProxy
	ActiveConnections int64
}

// IsAlive checks if the backend is alive (thread-safe)
func (b *Backend) IsAlive() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Alive
}

// SetAlive sets the backend alive status (thread-safe)
func (b *Backend) SetAlive(alive bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Alive = alive
}

// ServerPool manages all backends
type ServerPool struct {
	backends []*Backend
	mu       sync.RWMutex
	current  uint64 // for round-robin fallback
}

// AddBackend adds a new backend to the pool
func (s *ServerPool) AddBackend(backend *Backend) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backends = append(s.backends, backend)
}

// GetNextPeer returns the backend with the least active connections
// This implements the least connections load balancing strategy
func (s *ServerPool) GetNextPeer() *Backend {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var bestBackend *Backend
	var minConnections int64 = -1

	for _, b := range s.backends {
		if b.IsAlive() {
			// Load the connection count safely using atomic operations
			conn := atomic.LoadInt64(&b.ActiveConnections)

			// If this is the first healthy backend, or it has fewer connections
			// than our current best, we found a new best backend
			if minConnections == -1 || conn < minConnections {
				minConnections = conn
				bestBackend = b
			}
		}
	}

	return bestBackend
}

// MarkBackendStatus updates the alive status of a backend
func (s *ServerPool) MarkBackendStatus(backend *Backend, alive bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, b := range s.backends {
		if b.URL.String() == backend.URL.String() {
			b.SetAlive(alive)
			break
		}
	}
}

// HealthChecker performs periodic health checks on all backends
type HealthChecker struct {
	pool     *ServerPool
	interval time.Duration
}

// Start begins the health checking routine
func (hc *HealthChecker) Start() {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			hc.Check()
		}
	}()
}

// Check performs health checks on all backends
func (hc *HealthChecker) Check() {
	hc.pool.mu.RLock()
	backends := hc.pool.backends
	hc.pool.mu.RUnlock()

	for _, b := range backends {
		isAlive := isBackendAlive(b.URL)
		wasAlive := b.IsAlive()

		if isAlive != wasAlive {
			hc.pool.MarkBackendStatus(b, isAlive)
			status := "up"
			if !isAlive {
				status = "down"
			}
			log.Printf("Backend %s marked as %s\n", b.URL.String(), status)
		}
	}
}

// isBackendAlive checks if a backend is responsive
func isBackendAlive(url *url.URL) bool {
	timeout := time.Duration(2 * time.Second)
	conn, err := net.DialTimeout("tcp", url.Host, timeout)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

// GetAttemptsFromContext retrieves the number of attempts from the request context
func GetAttemptsFromContext(r *http.Request) int {
	if attempts, ok := r.Context().Value("attempts").(int); ok {
		return attempts
	}
	return 0
}

// IncrementAttemptsFromContext increments the attempts counter in the context
func IncrementAttemptsFromContext(r *http.Request) {
	if _, ok := r.Context().Value("attempts").(int); ok {
		r.Header.Add("X-Attempt", fmt.Sprintf("%d", GetAttemptsFromContext(r)+1))
	}
}

var serverPool ServerPool

// lb is the load balancer handler that routes requests to the least busy backend
func lb(w http.ResponseWriter, r *http.Request) {
	attempts := GetAttemptsFromContext(r)

	// Prevent infinite retry loops (max 3 attempts)
	if attempts > 3 {
		log.Printf("Max attempts reached for %s\n", r.RemoteAddr)
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Get the backend with the least active connections
	peer := serverPool.GetNextPeer()

	if peer == nil {
		log.Printf("No available backends for request from %s\n", r.RemoteAddr)
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Increment active connection count
	atomic.AddInt64(&peer.ActiveConnections, 1)
	defer atomic.AddInt64(&peer.ActiveConnections, -1)

	// Log the request routing
	log.Printf(
		"Routing request from %s to %s (active connections: %d)\n",
		r.RemoteAddr,
		peer.URL.String(),
		atomic.LoadInt64(&peer.ActiveConnections),
	)

	// Forward the request to the backend
	peer.ReverseProxy.ServeHTTP(w, r)
}

// NewBackend creates a new backend instance
func NewBackend(backendURL string) (*Backend, error) {
	parsedURL, err := url.Parse(backendURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse backend URL: %w", err)
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(parsedURL)

	// Add custom error handling for reverse proxy
	reverseProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Error forwarding request to backend: %v\n", err)

		// Increment attempts and retry with another backend
		attempts := GetAttemptsFromContext(r) + 1
		r.Header.Add("X-Attempt", fmt.Sprintf("%d", attempts))

		// Mark the backend as down if it's failing
		r.RequestURI = ""
		lb(w, r)
	}

	backend := &Backend{
		URL:          parsedURL,
		Alive:        true,
		ReverseProxy: reverseProxy,
	}

	return backend, nil
}

func main() {
	var backendServers []string
	backendServers = append(backendServers, "http://localhost:8001")
	backendServers = append(backendServers, "http://localhost:8002")
	backendServers, append(backendServers, "http://localhost:8003")

	// Initialize backends
	for _, backendURL := range backendServers {
		backend, err := NewBackend(backendURL)
		if err != nil {
			log.Fatalf("Failed to create backend: %v\n", err)
		}
		serverPool.AddBackend(backend)
	}

	// Start health checker
	healthChecker := &HealthChecker{
		pool:     &serverPool,
		interval: 10 * time.Second,
	}
	healthChecker.Start()

	// Set up the HTTP server
	server := &http.Server{
		Addr:         ":8080",
		Handler:      http.HandlerFunc(lb),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Load balancer started on %s\n", server.Addr)
	log.Printf("Configured backends: %d\n", len(serverPool.backends))

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server error: %v\n", err)
	}
}
