package structers

import (
	"net/url"
	"sync"
)

type Backend struct {
	URL   *url.URL
	Alive bool
	Mutex sync.RWMutex
}
