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

// Less is part of the heap.Interface and is used to compare two backends in the heap based on their current load.
// It returns true if the backend at index i has a lower current load than the backend at index j.
func (h BackendHeap) Less(i, j int) bool {
	return atomic.LoadInt64(&h[i].CurrentLoad) < atomic.LoadInt64(&h[j].CurrentLoad)
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
