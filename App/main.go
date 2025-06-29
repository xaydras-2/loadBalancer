package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/xaydras-2/loadBalancer/App/config"
	"github.com/xaydras-2/loadBalancer/App/functions"
	"github.com/xaydras-2/loadBalancer/App/structers"
)

func main() {

	// 1. Start initial replicas
	for i := 0; i < config.InitialReplicas; i++ {
		port := strconv.Itoa(config.StartPort + i)
		cid, be, err := functions.CreateReplicas(config.ImageName, config.ContainerPort, port)
		if err != nil {
			log.Fatalf("failed to create replica: %v", err)
		}
		config.Backends = append(config.Backends, be)
		config.Containers = append(config.Containers, cid)
	}

	// 2. Start auto-scaler
	go functions.AutoScaler()
	// 2.1 Start the active monitoring (AM) load balancer
	go functions.AMLB()

	// 3. HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// increment atomically
		atomic.AddInt64(&config.ReqCount, 1)

		// take snapshot under lock
		config.BackendsMu.Lock()
		snapshot := make([]*structers.Backend, len(config.Backends))
		copy(snapshot, config.Backends)
		config.BackendsMu.Unlock()

		functions.ProxyHandler(&snapshot)(w, r)
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// 4. Graceful shutdown setup
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Println("Load balancer running on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// if stop has been made then this block of code will work
	<-stop
	log.Println("Shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("Shutting down HTTP server…")
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}

	// 5. Tear down containers
	log.Println("Stopping backend containers…")
	for _, cid := range config.Containers {
		if msg, err := functions.CloseReplicas(cid); err != nil {
			log.Printf("error closing %s: %v", cid, err)
		} else {
			log.Printf("Container closed: %s", msg)
		}
	}

	log.Println("Clean exit")
}
