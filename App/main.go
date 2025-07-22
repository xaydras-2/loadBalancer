package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/xaydras-2/loadBalancer/App/config"
	"github.com/xaydras-2/loadBalancer/App/functions"
)

func main() {

	// 1. Start initial replicas
	functions.CallContainers()

	// 2. Start auto-scaler
	go functions.AutoScaler()
	// 2.1 Start the active monitoring (AM) load balancer
	go functions.AMLB()

	// start the health checking
	go functions.StartHealthChecker()

	// 3. HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// increment atomically
		atomic.AddInt64(&config.ReqCount, 1)
		functions.ProxyHandler()(w, r)
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

	go func() {
		for {
			config.BackendsMu.Lock()
			log.Printf("Healthy Backends in heap: %d", config.Backends.Len())
			for i, b := range config.Backends {
				log.Printf("  [%d] URL: %s, Alive: %t, Ill: %t, Load: %d",
					i, b.URL.String(), b.Alive, b.Ill, atomic.LoadInt64(&b.CurrentLoad))
			}
			log.Printf("Unhealthy Backends: %d", len(config.Unhealthy))
			for i, b := range config.Unhealthy {
				log.Printf("  [%d] URL: %s, Alive: %t, Ill: %t, Load: %d",
					i, b.URL.String(), b.Alive, b.Ill, atomic.LoadInt64(&b.CurrentLoad))
			}
			config.BackendsMu.Unlock()
			time.Sleep(5 * time.Second)
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
	for _, b := range config.Backends {
		if msg, err := functions.CloseReplicas(b.ContainerID); err != nil {
			log.Printf("error closing %s: %v", b.ContainerID, err)
		} else {
			log.Printf("Container closed: %s", msg)
		}
	}

	log.Println("Clean exit")
}
