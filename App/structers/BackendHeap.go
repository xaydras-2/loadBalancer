// Package structers defines core data structures used by the load balancer.
package structers

import (
	"sync/atomic"
)

// BackendHeap is a min-heap of Backend pointers, typically used to manage and prioritize backend servers
// based on custom criteria such as load, latency, or availability. It implements the heap.Interface
// and can be used with the standard library's "container/heap" package.
type BackendHeap []*Backend

// Len returns the number of backends in the heap.
func (h BackendHeap) Len() int {
	return len(h)
}

// Less compares two backends based first on health (alive backends before dead ones),
// then by current load (lower load has higher priority).
func (h BackendHeap) Less(i, j int) bool {
    bi, bj := h[i], h[j]

    // 1) Shutting‑down lanes last
    if bi.ShuttingDown != bj.ShuttingDown {
        return bi.ShuttingDown == 0 && bj.ShuttingDown == 1
    }

    // 2) Ill (failing probe) next worst
    if bi.Ill != bj.Ill {
        return !bi.Ill && bj.Ill
    }

    // 3) Alive vs dead
    if bi.Alive != bj.Alive {
        return bi.Alive && !bj.Alive
    }

    // 4) Both in same health/shutdown bucket → compare load
    return atomic.LoadInt64(&bi.CurrentLoad) < atomic.LoadInt64(&bj.CurrentLoad)
}


// Swap is part of the heap.Interface and is used to swap two backends in the heap.
// It updates the heap indices of the swapped backends.
func (h BackendHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].HeapIdx, h[j].HeapIdx = i, j
}

// Push adds a new backend to the heap. It sets the heap index of the backend to the
// current length of the heap before appending it. This function is part of the
// heap.Interface implementation, allowing the heap to grow dynamically as new backends are added.
func (h *BackendHeap) Push(x any) {
	b := x.(*Backend)
	b.HeapIdx = len(*h)
	*h = append(*h, b)
}

// Pop removes the root element (i.e. the backend with the lowest current load) from the heap and returns it.
// It updates the heap indices of the remaining backends after the removal. This function is part of the
// heap.Interface implementation, allowing the heap to shrink dynamically as backends are removed.
func (h *BackendHeap) Pop() any {
	old := *h
	n := len(old)
	b := old[n-1]
	b.HeapIdx = -1
	*h = old[:n-1]
	return b
}
