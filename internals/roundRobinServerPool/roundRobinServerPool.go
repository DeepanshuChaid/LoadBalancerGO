package roundRobinServerPool

import (
	"sync"

	"github.com/DeepanshuChaid/LoadBalancerGO.git/internals/backend"
)

type RoundRobinServerPool struct {
	backends []backend.Backend
	mux sync.RWMutex
	current int
}


func (s *RoundRobinServerPool) Rotate() backend.Backend {
	s.mux.Lock()

	s.current = (s.current + 1)// % s.GetServerPoolSize()

	s.mux.Unlock()

	return s.backends[s.current]
}

// THE INDEX OF THE CURRENT SERVER IS STORED IN AN INTEGER IN THE STRUCT IMPLEMENTATION
// THE ROTATE METHOD INCREMENTS THE CURRENT COUNT AND RETURNS THE NEXT SERVER ON THE LINE
// THEN THE GETNEXTVALIDPEER METHOD IS RESPONSIBLE FOR VALIDATING IF THE SERVER IS ALIVE AND ABLE TO RECIEVE REQUESTS IF THAT IS NOT THE CASE THE ITERATION CONTINUES UNTIL ONE IS FOUND
func (s *RoundRobinServerPool) GetNextValidPeer() backend.Backend {
	// for i := 0; i < s.GetServerPoolSize(); i++ {
	// 	nextPeer := s.Rotate()

	// 	if nextPeer.IsAlive() {
	// 		return nextPeer
	// 	}
	// }
	return nil
}

