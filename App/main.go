package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/xaydras-2/loadBalancer/App/functions"
	"github.com/xaydras-2/loadBalancer/App/structers"
)

var (
	backendsMu sync.Mutex
	backends   []*structers.Backend
	containers []string // real Docker container IDs
)

const (
	imageName          = "api_load_test:latest"
	containerPort      = "8080"
	startPort          = 9000
	initialReplicas    = 2
	maxReplicas        = 5
	minReplicas        = 1
	scaleUpThreshold   = 20 // requests per interval before creating a new container
	scaleDownThreshold = 5  // requests per interval before killing/stopping a container
	scaleInterval      = 15 * time.Second
)

var requestCount int

func main() {
	// 1. Start initial replicas
	for i := 0; i < initialReplicas; i++ {
		port := strconv.Itoa(startPort + i)
		containerID, backend, err := functions.CreateReplicas(imageName, containerPort, port)
		if err != nil {
			log.Fatalf("failed to create replica: %v", err)
		}
		backends = append(backends, backend)
		containers = append(containers, containerID)
	}

	// 2. Start auto-scaler
	go autoScaler()

	// 3. Create HTTP server with custom handler
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		backendsMu.Lock()
		requestCount++
		// snapshot current backends under lock
		snapshot := make([]*structers.Backend, len(backends))
		copy(snapshot, backends)
		backendsMu.Unlock()

		functions.ProxyHandler(&snapshot)(w, r)
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// 4. Listen for shutdown signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// 5. Run server in background
	go func() {
		log.Println("Load balancer running on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// 6. Block until a signal is received
	<-stop
	log.Println("Shutdown signal received")

	// 7. Gracefully shut down HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("Shutting down HTTP server…")
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}

	// 8. Tear down containers
	log.Println("Stopping backend containers…")
	for _, cid := range containers {
		if msg, err := functions.CloseReplicas(cid); err != nil {
			log.Printf("error closing %s: %v", cid, err)
		} else {
			log.Printf("Container closed: %s", msg)
		}
	}

	log.Println("Clean exit")
}

// autoScaler checks requestCount and scales up/down
func autoScaler() {
	for {
		time.Sleep(scaleInterval)
		backendsMu.Lock()
		count := requestCount
		requestCount = 0
		replicas := len(backends)

		if count > scaleUpThreshold && replicas < maxReplicas {
			// scale up
			port := strconv.Itoa(startPort + replicas)
			containerID, backend, err := functions.CreateReplicas(imageName, containerPort, port)
			if err != nil {
				log.Printf("scale up failed: %v", err)
			} else {
				backends = append(backends, backend)
				containers = append(containers, containerID)
				log.Printf("scaled up: now %d replicas", len(backends))
			}

		} else if count < scaleDownThreshold && replicas > minReplicas {
			// scale down
			lastIdx := len(backends) - 1
			cid := containers[lastIdx]
			if _, err := functions.CloseReplicas(cid); err != nil {
				log.Printf("scale down failed: %v", err)
			} else {
				backends = backends[:lastIdx]
				containers = containers[:lastIdx]
				log.Printf("scaled down: now %d replicas", len(backends))
			}
		}

		backendsMu.Unlock()
	}
}
