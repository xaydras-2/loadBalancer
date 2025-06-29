// --- functions/scale.go ---
package functions

import (
	"log"
	"strconv"

	"github.com/xaydras-2/loadBalancer/App/config"
)

// ScaleUp launches one more container and registers it.
func ScaleUp() {
	// figure out the new port by counting existing backends
	config.BackendsMu.Lock()
	port := strconv.Itoa(config.StartPort + len(config.Backends))
	config.BackendsMu.Unlock()

	cid, backend, err := CreateReplicas(
		config.ImageName,
		config.ContainerPort,
		port,
	)
	if err != nil {
		log.Printf("scale up failed: %v", err)
		return
	}

	config.BackendsMu.Lock()
	config.Backends = append(config.Backends, backend)
	config.Containers = append(config.Containers, cid)
	nowCount := len(config.Backends)
	config.BackendsMu.Unlock()

	log.Printf("scaled up: now %d replicas", nowCount)
}

// ScaleDown tears down the newest container—never going below MinReplicas.
func ScaleDown() {
	config.BackendsMu.Lock()
	defer config.BackendsMu.Unlock()

	if len(config.Backends) <= config.MinReplicas {
		log.Printf("cannot scale down below MinReplicas (%d)", config.MinReplicas)
		return
	}

	// pick the last container
	lastIdx := len(config.Backends) - 1
	cid := config.Containers[lastIdx]

	if _, err := CloseReplicas(cid); err != nil {
		log.Printf("scale down failed: %v", err)
		return
	}

	// shrink both slices
	config.Backends = config.Backends[:lastIdx]
	config.Containers = config.Containers[:lastIdx]
	log.Printf("scaled down: now %d replicas", len(config.Backends))
}
