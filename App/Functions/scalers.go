// Package functions implements core logic for active monitoring, load balancing,
// and auto-scaling of back-end services.
package functions

import (
	"container/heap"
	"log"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/xaydras-2/loadBalancer/App/config"
	"github.com/xaydras-2/loadBalancer/App/structers"
)

var (
	// this used to prevent race condition so that there isn't a multiple creation/deletion at the same
	scalingMutex sync.Mutex
)

// ScaleUp launches one more container and registers it.
func ScaleUp() {

	scalingMutex.Lock()
	defer scalingMutex.Unlock()

	backend, err := CreateReplicas(
		config.ImageName,
		config.ContainerPort,
		config.NetworkName,
	)
	if err != nil {
		log.Printf("scale up failed: %v", err)
		return
	}

	// mark it as “not ready yet”
	backend.Alive = false
	backend.Ill = true

	config.BackendsMu.Lock()
	total := config.Backends.Len() + len(config.Unhealthy)
	if total >= config.MaxReplicas {
		config.BackendsMu.Unlock()
		log.Printf("cannot scale up beyond MaxReplicas (%d)", config.MaxReplicas)
		CloseReplicas(backend.ContainerID)
		return
	}

	config.Unhealthy = append(config.Unhealthy, backend)
	nowTotal := total + 1
	config.BackendsMu.Unlock()

	// 2) fire the immediate‐check event (non‑blocking)
	select {
	case config.NewBackendTrigger <- backend:
	default:
		// if buffer is full, we can safely drop it—
		// the next ticker tick will still check it.
	}

	log.Printf("scale up in progress: now %d total replicas (pending: %d)", nowTotal, len(config.Unhealthy))
}

// ScaleDown tears down the newest container—never going below MinReplicas.
func ScaleDown() {

	scalingMutex.Lock()
	defer scalingMutex.Unlock()

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

	removeFromUnHealthy(b)

	// Mark as shutting down to prevent new requests
	atomic.StoreInt32(&b.ShuttingDown, 1)

	b.Alive = false

	// Wait for in-flight requests to finish
	// get the current load
	currentLoad := atomic.LoadInt64(&b.CurrentLoad)

	if currentLoad > 0 {
		log.Printf("backend %s has %d active requests, cannot scale down now",
			b.ContainerID, currentLoad)
		atomic.StoreInt32(&b.ShuttingDown, 0)
		heap.Push(&config.Backends, b)
		return
	}

	// Backend has no active requests, safe to shut down
	if _, err := CloseReplicas(b.ContainerID); err != nil {
		// If the error is "No such container", just log and do NOT re-add to heap
		if strings.Contains(err.Error(), "No such container") {
			log.Printf("container %s already removed, skipping re-add", b.ContainerID)
			return
		}
		log.Printf("scale down failed: %v", err)
		removeFromUnHealthy(b)
		// Put backend back if shutdown failed
		heap.Push(&config.Backends, b)
		return
	}

	log.Printf("scaled down: removed %q, now %d replicas", b.ContainerID, config.Backends.Len())
}

func removeFromUnHealthy(b *structers.Backend) {
	for i, ub := range config.Unhealthy {
		if ub == b {
			heap.Remove(&config.Unhealthy, i)
			break
		}
	}
}
