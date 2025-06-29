package functions

import (
	"log"
	"time"

	"github.com/xaydras-2/loadBalancer/App/config"
	"golang.org/x/exp/slices"
)

//active monitoring (AM) load balancer
func AMLB() {
	ticker := time.NewTicker(config.ScaleInterval)
	defer ticker.Stop()

	for range ticker.C {
		// 1) Snapshot current state under lock
		config.BackendsMu.Lock()
		currentReplicas := len(config.Backends)
		containerIDs := slices.Clone(config.Containers)
		config.BackendsMu.Unlock()

		// Skip stats collection if there are no containers
		if len(containerIDs) == 0 {
			log.Printf("no containers to monitor")
			continue
		}

		// 2) Gather metrics (no lock)
		type stats struct {
			cpuPct, memPct float64
			err            error
		}
		results := make([]stats, len(containerIDs))
		for i, cid := range containerIDs {
			if cid == "" {
				log.Printf("skipping empty container ID at index %d", i)
				continue
			}
			info, err := GetInfoAboutReplica(cid)

			if err != nil {
				log.Printf("error getting stats for container %s: %v", cid, err)
				results[i] = stats{
					cpuPct: 0,
					memPct: 0,
					err:    err,
				}
				continue
			}

			// Check if info is nil to prevent panic
			if info == nil {
				log.Printf("received nil info for container %s", cid)
				results[i] = stats{
					cpuPct: 0,
					memPct: 0,
					err:    err,
				}
				continue
			}
			results[i] = stats{
				cpuPct: info.CPUPercent,
				memPct: info.MemoryPercent,
				err:    err,
			}
		}

		// 3) Determine if we need to scale up or down once
		wantScaleUp := false
		wantScaleDown := false
		validContainers := 0

		for _, r := range results {
			if r.err != nil {
				// log and skip this container
				log.Printf("failed to inspect %+v: %v", r, r.err)
				continue
			}
			if r.cpuPct > 75.0 || r.memPct > 80.0 {
				wantScaleUp = true
			}
			if r.cpuPct < 40.0 && r.memPct < 50.0 {
				wantScaleDown = true
			}
			// if both become true, you might choose to prioritize one
		}

		// Don't make scaling decisions if we have no valid containers
		if validContainers == 0 {
			continue
		}

		// 4) Enforce min/max replicas and then perform single action
		if wantScaleUp && currentReplicas < config.MaxReplicas {
			go func(replicaIndex int) {
				// lock only around mutation of shared slices
				config.BackendsMu.Lock()
				defer config.BackendsMu.Unlock()
				go ScaleUp()
			}(currentReplicas)
		} else if wantScaleDown && currentReplicas > config.MinReplicas {
			go func() {
				config.BackendsMu.Lock()
				defer config.BackendsMu.Unlock()
				go ScaleDown()
			}()
		}
	}
}
