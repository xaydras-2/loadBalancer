package structers

import (
	"net/http"
	"net/url"
	"sync"
)
type Backend struct {
  URL    *url.URL
  Alive  bool
  Mutex  sync.RWMutex
}

func (b *Backend) CheckHealth() {
  resp, err := http.Get(b.URL.String() + "/healthz")
  b.Mutex.Lock()
  b.Alive = (err == nil && resp.StatusCode == 200)
  b.Mutex.Unlock()
}