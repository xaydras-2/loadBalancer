package functions

import (
	"container/heap"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xaydras-2/loadBalancer/App/config"
	"github.com/xaydras-2/loadBalancer/App/structers"
)

var (
	httpClient = &http.Client{
		Timeout: 500 * time.Millisecond,
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
	// skip if unAlive
	if !b.Alive {
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
	for range ticker.C {
		config.BackendsMu.Lock()
		backends := append([]*structers.Backend{}, config.Backends...)
		config.BackendsMu.Unlock()

		// check all in parallel
		var wg sync.WaitGroup
		toRemove := make(chan *structers.Backend, len(backends))
		for _, b := range backends {
			wg.Add(1)
			go func(b *structers.Backend) {
				defer wg.Done()
				ok, _, _ := checkAlive(b)
				if !ok {
					toRemove <- b
				}
			}(b)
		}
		wg.Wait()
		close(toRemove)

		// remove dead ones under lock
		config.BackendsMu.Lock()
		for deadBackend := range toRemove {
			// Find the backend in the current heap by URL AND verify it still exists
			found := false
			for i, currentBackend := range config.Backends {
				if currentBackend.URL.String() == deadBackend.URL.String() &&
					currentBackend.ContainerID == deadBackend.ContainerID {
					heap.Remove(&config.Backends, i)
					found = true
					break
				}
			}
			if !found {
				log.Printf("Backend %v already removed by another process", deadBackend.URL)
			}
		}
		config.BackendsMu.Unlock()
	}

}

func pickBackendAndIncrement() *structers.Backend {
	config.BackendsMu.Lock()
	defer config.BackendsMu.Unlock()

	if config.Backends.Len() == 0 {
		return nil
	}

	b := config.Backends[0] // cheapest load
	atomic.AddInt64(&b.CurrentLoad, 1)
	heap.Fix(&config.Backends, b.HeapIdx)

	return b
}

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

		proxy := &httputil.ReverseProxy{Director: director}
		proxy.ServeHTTP(w, r)
	}
}
