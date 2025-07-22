// Package config holds global configuration and runtime state for the load balancer.
// It includes synchronization primitives, application constants, and shared variables
// used across the application.
package config

import (
	"sync"
	"time"

	composeTypes "github.com/compose-spec/compose-go/types"
	"github.com/xaydras-2/loadBalancer/App/structers"
)

var (
	// BackendsMu protects concurrent access to the Backends heap.
	BackendsMu sync.Mutex

	// Backends is a min-heap of available back-end servers, ordered by load or other criteria.
	Backends structers.BackendHeap

	// ReqCount is an atomic counter of total requests processed,
	// used to determine scaling decisions based on request volume.
	ReqCount int64

	// NetworkName holds the Docker network name for service discovery.
	NetworkName string

	// SvcTemp is a template for Docker Compose service configuration,
	// it holds temporary the current svc, or the want one.
	// used when scaling containers up or down.
	SvcTemp composeTypes.ServiceConfig

	// Unhealthy tracks back-end servers marked as unhealthy and pending recovery.
	Unhealthy structers.BackendHeap

	// NewBackendTrigger is a buffered channel used to notify the health‐checker
	// whenever a new backend is added, so it can perform an immediate health probe
	// instead of waiting for the next periodic tick.
	NewBackendTrigger = make(chan *structers.Backend, 10)
)

const (
	// ParentName is the base name for the load balancer service.
	ParentName = "api"

	// ImageName specifies the Docker image tag used for load testing.
	ImageName = "api_load_test:latest"

	// ContainerPort is the port on which back-end containers listen internally.
	ContainerPort = "8080"

	// InitialReplicas defines the number of back-end instances at startup.
	InitialReplicas = 2

	// MaxReplicas sets the upper bound for auto-scaling.
	MaxReplicas = 5

	// MinReplicas sets the lower bound for auto-scaling.
	MinReplicas = 1

	// ScaleUpThreshold is the number of requests per interval
	// that triggers scaling up additional replicas.
	ScaleUpThreshold = 20 // requests per interval

	// ScaleDownThreshold is the number of requests per interval
	// that triggers scaling down replicas.
	ScaleDownThreshold = 5 // requests per interval

	// ScaleInterval is the duration when the x function work to make a decision.
	// putting it simply: "for every n sec do this"
	ScaleInterval = 15 * time.Second

	// ScaleInterval is the duration when the x function work to make a decision. This one is used for AM(Active Monitoring).
	// putting it simply: "for every n sec do this"
	ScaleIntervalAM = 33 * time.Second

	// DockerComposePath points to the Docker Compose file
	// used to spawn and manage containers.
	DockerComposePath = "../API/docker-compose.yaml"

	// StartupGracePeriod indicates the period in which an x api must be full woken up
	StartupGracePeriod = 10 * time.Second
)
