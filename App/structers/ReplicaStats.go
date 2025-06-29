package structers

type ReplicaStats struct {
	CPUPercent    float64 // CPU usage %
	MemoryUsage   uint64  // Memory used in bytes
	MemoryLimit   uint64  // Memory limit in bytes
	MemoryPercent float64 // Memory usage %
}
