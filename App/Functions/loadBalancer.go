// Package functions implements core logic for active monitoring, load balancing,
// and auto-scaling of back-end services.
package functions

import (
	"container/heap"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/xaydras-2/loadBalancer/App/config"
	"github.com/xaydras-2/loadBalancer/App/structers"
)

var (
	httpClient = &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     30 * time.Second,
			DisableKeepAlives:   false, // ensure keep-alive
		},
	}
)

// checkAlive sends a GET request to the given backend's /healthz page and
// returns whether the response was successful (200-399), the latency of the
// request, and any error that occurred.
func checkAlive(b *structers.Backend) (bool, time.Duration, error) {
	// skip if dead
	if !b.Alive && !b.Ill {
		return false, 0, nil
	}

	if atomic.LoadInt32(&b.ShuttingDown) == 1 {
		return false, 0, nil
	}

	healthURL := b.URL.ResolveReference(&url.URL{Path: "/healthz"})
	req, err := http.NewRequest(http.MethodGet, healthURL.String(), nil)
	if err != nil {
		return false, 0, err
	}
	start := time.Now()
	resp, err := httpClient.Do(req)
	latency := time.Since(start)

	if err != nil {
		return false, latency, err
	}

	// drain the body so the connection can be reused cleanly
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp.StatusCode < 400, latency, nil
}

func StartHealthChecker() {
	ticker := time.NewTicker(config.ScaleInterval)
	defer ticker.Stop()

	// keeping the same “snapshot” pattern for the periodic scan.
	for {
		select {
		case <-ticker.C:
			runAllChecks()
		case b := <-config.NewBackendTrigger:
			// immediately check this one new backend
			checkAndReheap(b)
		}
	}
}

func appendIfNewUnhealthy(b *structers.Backend) {
	for _, ub := range config.Unhealthy {
		if ub == b {
			return // already marked
		}
	}
	config.Unhealthy = append(config.Unhealthy, b)
}

func runAllChecks() {
	backends := snapshotAllBackends()

	for _, b := range backends {
		ok, _, _ := checkAlive(b)

		config.BackendsMu.Lock()

		if ok {
			handleRecoveredBackend(b)
		} else {
			// Health check failed
			if !b.Alive {
				// Already dead, just add to unhealthy if not already there
				appendIfNewUnhealthy(b)
			} else {
				// Still marked as alive but failing health check
				handleFailingBackend(b)
				// Only add to unhealthy if it's now dead or ill
				if !b.Alive || b.Ill {
					appendIfNewUnhealthy(b)
				}
			}
		}

		config.BackendsMu.Unlock()
	}
}

func checkAndReheap(b *structers.Backend) {
	log.Printf("checking the backend b: %v", b)
	ok, _, _ := checkAlive(b)

	config.BackendsMu.Lock()
	defer config.BackendsMu.Unlock()

	if ok {
		handleRecoveredBackend(b)
		return
	}

	// Health check failed
	if !b.Alive {
		// Already marked dead, just collect it
		appendIfNewUnhealthy(b)
		return
	}

	// Still alive but failing health check
	handleFailingBackend(b)

	// Add to unhealthy if now dead or ill
	if !b.Alive || b.Ill {
		appendIfNewUnhealthy(b)
	}
}

// pickBackendAndIncrement it peaks the first backend of the heap, since the backend heap is auto ordered by less,
// which make it the less loaded one.
func pickBackendAndIncrement() *structers.Backend {
	config.BackendsMu.Lock()
	defer config.BackendsMu.Unlock()

	for config.Backends.Len() > 0 {
		b := config.Backends[0]

		if b.HeapIdx != 0 {
			log.Printf("Warning: Heap inconsistency detected for %s (expected index 0, got %d)",
				b.URL.String(), b.HeapIdx)
			heap.Init(&config.Backends) // Rebuild heap
			continue
		}

		if !b.Alive || b.Ill || atomic.LoadInt32(&b.ShuttingDown) == 1 {
			heap.Pop(&config.Backends)
			appendIfNewUnhealthy(b)
			continue
		}
		// found a healthy one
		atomic.AddInt64(&b.CurrentLoad, 1)
		heap.Fix(&config.Backends, b.HeapIdx)
		return b
	}
	return nil
}

// ProxyHandler it handles the traffic and direct it to the selected backend from pickBackendAndIncrement
// and passes the request to it
func ProxyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if config.Backends == nil || config.Backends.Len() == 0 {
			http.Error(w, "no backends available", http.StatusServiceUnavailable)
			return
		}

		// Pick backend and increment load atomically
		b := pickBackendAndIncrement()
		if b == nil {
			http.Error(w, "no backends available", http.StatusServiceUnavailable)
			return
		}

		// Ensure load is decremented when request completes
		defer func() {
			atomic.AddInt64(&b.CurrentLoad, -1)

			config.BackendsMu.Lock()
			// Verify backend still exists in heap before fixing
			if b.HeapIdx >= 0 && b.HeapIdx < config.Backends.Len() {
				heap.Fix(&config.Backends, b.HeapIdx)
			}
			config.BackendsMu.Unlock()
		}()

		director := func(req *http.Request) {
			req.URL.Scheme = b.URL.Scheme
			req.URL.Host = b.URL.Host
			req.Host = b.URL.Host
		}

		proxy := &httputil.ReverseProxy{
			Director: director,
			ErrorHandler: func(rw http.ResponseWriter, req *http.Request, err error) {
				log.Printf("Proxy error for %s %s: %v", req.Method, req.URL.String(), err)
				http.Error(rw, "Proxy error: "+err.Error(), http.StatusBadGateway)
			},
		}
		proxy.ServeHTTP(w, r)

	}
}
