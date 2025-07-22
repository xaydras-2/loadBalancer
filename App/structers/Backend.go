// Package structers defines core data structures used by the load balancer.
package structers

import (
	"net/url"
	"sync"
	"time"
)

// Backend represents a single back-end server instance in the pool.
// It holds connection details, health status, and load information.
type Backend struct {
	// URL is the network address of the back-end container.
	URL *url.URL

	// Alive indicates whether the back-end is currently healthy and reachable.
	Alive bool

	// Ill indicates if a container is having problems responding to a check or a request, which might cause lost requests.
	Ill bool

	// CurrentLoad tracks the number of active requests or load metric.
	CurrentLoad int64

	//
	ShuttingDown int32

	// HeapIdx is the index of this backend entry in the heap, used for reordering.
	HeapIdx int

	// ContainerID is the unique identifier assigned by Docker or the container runtime.
	ContainerID string

	// Mutex guards concurrent access to fields like Alive and CurrentLoad.
	Mutex sync.RWMutex

	// StartTime save the time of initialization of a container,
	// used to give a warm up phase to an api to start up
	StartTime time.Time
}
