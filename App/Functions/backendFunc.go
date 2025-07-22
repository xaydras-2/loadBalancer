package functions

import (
	"container/heap"
	"log"
	"time"

	"github.com/xaydras-2/loadBalancer/App/config"
	"github.com/xaydras-2/loadBalancer/App/structers"
)

func snapshotAllBackends() []*structers.Backend {
	config.BackendsMu.Lock()
	defer config.BackendsMu.Unlock()

	all := make([]*structers.Backend, 0, config.Backends.Len()+len(config.Unhealthy))
	all = append(all, config.Backends...)
	all = append(all, config.Unhealthy...)

	config.Unhealthy = nil
	return all
}

func handleRecoveredBackend(b *structers.Backend) {
	if !b.Alive || b.Ill {
		b.Alive = true
		b.Ill = false
		// // Only add to heap if not already there
		// if b.HeapIdx < 0 || b.HeapIdx >= config.Backends.Len() {
		// 	heap.Push(&config.Backends, b)
		// } else {
		// 	// Already in heap, just fix position
		// 	heap.Fix(&config.Backends, b.HeapIdx)
		// }
		heap.Push(&config.Backends, b)
	}
}
func handleFailingBackend(b *structers.Backend) {
	if !b.Ill {
		// first failure -> go “ill”
		b.Ill = true
		log.Printf("Backend %s marked as ill", b.URL.String())

		// Fix heap position since Ill status affects ordering
		if b.HeapIdx >= 0 && b.HeapIdx < config.Backends.Len() {
			heap.Fix(&config.Backends, b.HeapIdx)
		}
	} else {
		// second consecutive fail -> only kill if grace has passed
		if time.Since(b.StartTime) < config.StartupGracePeriod {
			log.Printf(
				"Backend %s still starting (%.0fs), postponing death",
				b.URL.String(),
				time.Since(b.StartTime).Seconds(),
			)

		} else {
			b.Alive = false
			b.Ill = false
			log.Printf("Backend %s marked as dead", b.URL.String())
			// Remove from active heap since it's now dead
			if b.HeapIdx >= 0 && b.HeapIdx < config.Backends.Len() {
				heap.Remove(&config.Backends, b.HeapIdx)
			}
			// It will be added to unhealthy list by the caller
		}
	}

	// fix heap so its position / LOD updates
	heap.Fix(&config.Backends, b.HeapIdx)
}
