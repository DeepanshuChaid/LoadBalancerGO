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

func (s *ServerPool) GetNextPeer() *Backend {
	s.mu.Lock()
	defer s.mu.Unlock()

	var bestBackend *Backend
	var minConnections int64 = -1

	for _, b := range s.backends {
		if b.IsAlive() {
			// Load the connections safely using atomic operations
			conn := atomic.LoadInt64(&b.ActiveConnections)

			// IF THIS IS THE FIRST HEALTHY BACKEND OR IT HAS FEWER CONNECTIONS
			// THAN OUR CURRENT BEST WE FOUND A NEW BEST BACKEND
			if minConnections == -1 || conn < minConnections {
				minConnections = conn
				bestBackend = b
			}
		}
	}
	return bestBackend
}


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

// HEALTHCHECKER PERFORMS PERIODIC HEALTH CHECKS ON ALL BACKENDS
type HealthChecker struct {
	pool *ServerPool
	interval time.Duration
}

func (hc *HealthChecker) Start() {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	go func () {
		for range ticker.C {
			hc.Check()
		}
	}()
}

// PERFORM HEALTH CHECKS ON ALL OF THE BACKENDS
func (hc *HealthChecker) Check () {
	hc.pool.mu.RLock()
	backends := hc.pool.backends
	hc.pool.mu.RUnlock()

	for _, b := range backends {
		isAlive := isBackendAlive(b.URL)
		wasAlive := b.IsAlive()

		if wasAlive != isAlive {
			hc.pool.MarkBackendStatus(b, isAlive)
			status := "up"
			if !isAlive {
				status = "down"
			}
			log.Printf("Backend %s marked as %s \n", b.URL.String(), status)
		}
	}
}

// CHECKS IF THE BACKENDS REPONDS OR NOT
func isBackendAlive(url *url.URL) bool  {
	timeout  := time.Duration(2*time.Second)
	conn, err := net.DialTimeout("tcp", url.Host, timeout)
	if err != nil {
		return false
	}

	defer conn.Close()

	return true
}

func GetAttemptsFromContext(r *http.Request) int {
	if attempts, ok := r.Context().Value("attempts").(int); ok {
		return attempts
	}
	return 0
}

// IncrementAttemptsFromContext increments the attempts counter is the context
func IncrementAttemptsFromContext(r *http.Request) {
	if _, ok := r.Context().Value("attempts").(int); ok {
		r.Header.Add("X-Attempt", fmt.Sprintf("%d", GetAttemptsFromContext(r)+1))
	}
}

var serverPool ServerPool

// THIS IS THE LOAD BALANCE HANDLER THAT ROUTES REQUESTS TO THE LEAST BUSY BACKEND
func LoadBalance(w http.ResponseWriter, r *http.Request) {
	attempts := GetAttemptsFromContext(r)

	if attempts > 3 {
		log.Printf("Max attempts reached for %s\n", r.RemoteAddr)
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	peer := serverPool.GetNextPeer()

	if peer == nil {
		log.Printf("No available backends for request from %s\n", r.RemoteAddr)
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	atomic.AddInt64(&peer.ActiveConnections, 1)
	defer atomic.AddInt64(&peer.ActiveConnections, -1)

	log.Printf(
		"Routing request from %s to %s (active connections: %d) \n",
		r.RemoteAddr,
		peer.URL.String,
		atomic.LoadInt64(&peer.ActiveConnections),
	)

	// SERVEHTTP FORWARDS THE REQUEST AND GIVES ALL THE INFO FOR THE HTTP REQUEST TO TAKE PLACE SUCH AS REQ BODY AND HEADER COOKIES ETC
	peer.ReverseProxy.ServeHTTP(w, r)
}

// 	CREATE A NEW BACKEND
func NewBackend(backendURL string) (*Backend, error) {
	parsedUrl, err := url.Parse(backendURL)

	if err != nil {
		return nil, fmt.Errorf("Failed to parse backend url: %w", err)
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(parsedUrl)

	reverseProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Error forwarding request to backend:", err)

		// INCREMENT ATTEMPTS AND RETRY WITH ANOTHER BACKEND
		attempts := GetAttemptsFromContext(r) + 1

		r.Header.Add("X-Attempt", fmt.Sprintf("%d", attempts))

		// MARK THE BACKEND AS DOWN IF ITS FAILING
		r.RequestURI = ""
		LoadBalance(w, r)
	}

	backend := &Backend{
		URL: parsedUrl,
		Alive: true,
		ReverseProxy: reverseProxy,
	}

	return backend, nil
}


func main() {
	var backendServers = []string{
		"http://localhost:8001",
		"http://localhost:8002",
		"http://localhost:8003",
	}

	for _, baeckendUrl := range backendServers {
		backend, err := NewBackend(baeckendUrl)
		if err != nil {
			log.Fatal("Failed to create backend:", err)
		}

		serverPool.AddBackend(backend)
	}

	healthChecker := &HealthChecker{
		pool: &serverPool,
		interval: 30 * time.Second,
	}

	// Start HEALTH CHECKER
	healthChecker.Start()

	// SET UP THE HTTP SERVER
	server := http.Server{
		Addr: ":3000",
		Handler: http.HandlerFunc(LoadBalance),
		ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout: 120 * time.Second,
	}

	log.Printf("Load balancer started on %s\n", server.Addr)
	log.Printf("Configured backends: %d\n", len(serverPool.backends))

	if err := server.ListenAndServe(); err != nil {
		log.Fatal("ERROR: ", err)
	}
}
