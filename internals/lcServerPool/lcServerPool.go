package lcServerPool

import (
	"sync"

	"github.com/DeepanshuChaid/LoadBalancerGO.git/internals/backend"
)

type lcServerPool struct {
	backends []backend.Backend
	mux sync.RWMutex
}

// a load balancer interface and underlying struct have been implemented as a wrapper of the server pool
// ??? I DID NOT UNDERSTAND ANYTHING AT ALL FOR NOW
func (s *lcServerPool) GetNextValidPeer() backend.Backend {
	var leastConnectedPeer backend.Backend

	// FIND ATLEAST ONE VALID PEER
	for _, b := range s.backends {
		if b.IsAlive() {
			leastConnectedPeer = b
			break
		}
	}

	// CHECK WHICH ONE HAS THE LEST NUMBER OF THE AVTIVE CONNECTIONS
	for _, b := range s.backends {
		if !b.IsAlive() {
			continue
		}
		if leastConnectedPeer.GetActiveConnections() > b.GetActiveConnections() {
			leastConnectedPeer = b
		}
	}
	return leastConnectedPeer
}
