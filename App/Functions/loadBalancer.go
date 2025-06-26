package functions

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

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
	rrCounter int64
)

// checkAlive sends a GET request to the given backend's /healthz page and
// returns whether the response was successful (200-399), the latency of the
// request, and any error that occurred.
func checkAlive(b *structers.Backend) (bool, time.Duration, error) {
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

// pickAliveBackends filters only healthy ones.
func pickAliveBackends(bs []*structers.Backend) []*structers.Backend {
	var aliveList []*structers.Backend
	for i := range bs {
		ok, lat, err := checkAlive(bs[i])
		if ok {
			aliveList = append(aliveList, bs[i])
		} else {
			log.Printf("↓ %s (err: %v, latency: %v)\n", bs[i].URL, err, lat)
		}
	}
	return aliveList
}

// pickBackend does round-robin among healthy backends.
func pickBackend(bs []*structers.Backend) *structers.Backend {
	alive := pickAliveBackends(bs)
	if len(alive) == 0 {
		return nil
	}
	idx := atomic.AddInt64(&rrCounter, 1) % int64(len(alive))
	b := alive[idx]
	if atomic.LoadInt64(&rrCounter)%100 == 0 {
		go TraceLatency(b.URL.String())
	}
	return b
}

// ProxyHandler forwards the incoming request to one of the backends.
func ProxyHandler(backends *[]*structers.Backend) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b := pickBackend(*backends)
		if b == nil {
			http.Error(w, "no backends available", http.StatusServiceUnavailable)
			return
		}
		director := func(req *http.Request) {
			req.URL.Scheme = b.URL.Scheme
			req.URL.Host = b.URL.Host
			req.Host = b.URL.Host
		}
		proxy := &httputil.ReverseProxy{Director: director}
		proxy.ServeHTTP(w, r)
	}
}

// TraceLatency writes a detailed timing breakdown into Logs/latency.log.
func TraceLatency(url string) {
	var dnsStart, connStart, gotConn time.Time
	trace := &httptrace.ClientTrace{
		DNSStart:     func(_ httptrace.DNSStartInfo) { dnsStart = time.Now() },
		ConnectStart: func(_, _ string) { connStart = time.Now() },
		GotConn:      func(_ httptrace.GotConnInfo) { gotConn = time.Now() },
	}
	ctx := httptrace.WithClientTrace(context.Background(), trace)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

	start := time.Now()
	resp, err := httpClient.Do(req)
	total := time.Since(start)
	if resp != nil {
		resp.Body.Close()
	}

	logDir := filepath.Join("Logs")
	os.MkdirAll(logDir, os.ModePerm)
	f, ferr := os.OpenFile(filepath.Join(logDir, "latency.log"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if ferr != nil {
		log.Printf("cannot open latency log: %v", ferr)
		return
	}
	defer f.Close()

	logger := log.New(f, "", log.LstdFlags)
	logger.Printf("url=%s total=%v dns=%v conn=%v ttfb=%v err=%v\n",
		url,
		total,
		connStart.Sub(dnsStart),
		gotConn.Sub(connStart),
		time.Since(gotConn),
		err,
	)
}
