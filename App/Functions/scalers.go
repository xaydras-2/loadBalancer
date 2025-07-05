// --- functions/scale.go ---
package functions

import (
	"container/heap"
	"log"
	"sync/atomic"

	"github.com/xaydras-2/loadBalancer/App/config"
	"github.com/xaydras-2/loadBalancer/App/structers"
)

// ScaleUp launches one more container and registers it.
func ScaleUp() {

	backend, err := CreateReplicas(
		config.ImageName,
		config.ContainerPort,
	)
	if err != nil {
		log.Printf("scale up failed: %v", err)
		return
	}

	config.BackendsMu.Lock()
	heap.Push(&config.Backends, backend)
	nowCount := config.Backends.Len()
	config.BackendsMu.Unlock()

	log.Printf("scaled up: now %d replicas", nowCount)
}

// ScaleDown tears down the newest container—never going below MinReplicas.
func ScaleDown() {
	config.BackendsMu.Lock()
	defer config.BackendsMu.Unlock()

	if config.Backends.Len() <= config.MinReplicas {
		log.Printf("cannot scale down below MinReplicas (%d)", config.MinReplicas)
		return
	}

	if config.Backends.Len() == 0 {
		log.Printf("no backends available to scale down")
		return
	}

	// Pop the least-loaded backend from the heap
	b := heap.Pop(&config.Backends).(*structers.Backend)

	b.Alive = false

	// Wait for in-flight requests to finish
	// get the current load
	currentLoad := atomic.LoadInt64(&b.CurrentLoad)

	if currentLoad > 0 {
		log.Printf("backend %s has %d active requests, cannot scale down now",
			b.ContainerID, currentLoad)
		heap.Push(&config.Backends, b)
		return
	}

	// Backend has no active requests, safe to shut down
	if _, err := CloseReplicas(b.ContainerID); err != nil {
		log.Printf("scale down failed: %v", err)
		// Put backend back if shutdown failed
		heap.Push(&config.Backends, b)
		return
	}

	log.Printf("scaled down: removed %q, now %d replicas", b.ContainerID, config.Backends.Len())
}
