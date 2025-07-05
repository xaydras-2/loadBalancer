package graphs

import(
	"context"
	"log"
	"net/http"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"time"
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

// TODO: turn this in to a real life trafic graph

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
