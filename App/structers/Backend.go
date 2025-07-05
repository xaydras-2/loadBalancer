package structers

import (
	"net/url"
	"sync"
)

type Backend struct {
	URL         *url.URL
	Alive       bool
	CurrentLoad int64
	HeapIdx     int
	ContainerID string
	Mutex       sync.RWMutex
}
