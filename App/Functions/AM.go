// Package functions implements core logic for active monitoring, load balancing,
// and auto-scaling of back-end services.
package functions

import (
	"fmt"
	"log"
	"time"

	"github.com/xaydras-2/loadBalancer/App/config"
)

type stats struct {
	cpuPct, memPct float64
	err            error
}

// Active Monitoring Load Balancer (AMLB) periodically monitors the containers and decides whether to scale up or down
// based on CPU and memory usage. It enforces the minimum and maximum replica counts
// and performs a single scaling action at a time. The check is done every ScaleInterval (default is 15s).
func AMLB() {
	ticker := time.NewTicker(config.ScaleIntervalAM)
	defer ticker.Stop()

	log.Printf("i'm called AMLB")

	for range ticker.C {
		// 1) Snapshot current state under lock
		config.BackendsMu.Lock()
		currentReplicas := config.Backends.Len()
		containerIDs := make([]string, config.Backends.Len())
		for i, be := range config.Backends {
			containerIDs[i] = be.ContainerID
		}
		config.BackendsMu.Unlock()

		// Skip stats collection if there are no containers
		if len(containerIDs) == 0 {
			log.Printf("no containers to monitor")
			continue
		}

		results := make([]stats, len(containerIDs))

		appendResults(containerIDs, &results)

		// 3) Determine if we need to scale up or down once
		scaleUpCount := 0
		scaleDownCount := 0
		// i created this var just as a guard, i already start at n InitialReplicas
		validCont := 0

		for _, r := range results {
			if r.err != nil {
				// log and skip this container
				log.Printf("failed to inspect %+v: %v", r, r.err)
				continue
			}

			validCont++

			if r.cpuPct > 75.0 || r.memPct > 80.0 {
				scaleUpCount++
			}
			if r.cpuPct < 40.0 && r.memPct < 50.0 {
				scaleDownCount++
			}
		}

		if validCont == 0 {
			log.Printf("no valid containers")
			continue
		}

		upRatio := float64(scaleUpCount) / float64(validCont)
		downRatio := float64(scaleDownCount) / float64(validCont)

		if upRatio > 0.6 && currentReplicas < config.MaxReplicas {
			ScaleUp()
		} else if downRatio > 0.8 && currentReplicas > config.MinReplicas {
			ScaleDown()
		}
	}
}

// appendResults iterates over a list of container IDs, collects CPU and memory
// usage statistics for each container by calling GetInfoAboutReplica, and
// stores the results in the provided stats slice. If an error occurs while
// fetching stats or the container ID is empty, it logs the error and sets
// default values for the CPU and memory percentages in the results slice.
// This function is intended to run concurrently and updates the results slice in place.
func appendResults(containerIDs []string, results *[]stats) {
	for i, cid := range containerIDs {
		if cid == "" {
			log.Printf("skipping empty container ID at index %d", i)
			(*results)[i] = stats{
				cpuPct: 0,
				memPct: 0,
				err:    fmt.Errorf("empty container ID"),
			}
			continue
		}
		info, err := GetInfoAboutReplica(cid)

		if err != nil {
			log.Printf("error getting stats for container %s: %v", cid, err)
			(*results)[i] = stats{
				cpuPct: 0,
				memPct: 0,
				err:    err,
			}
			continue
		}

		// Check if info is nil to prevent panic
		if info == nil {
			log.Printf("received nil info for container %s", cid)
			(*results)[i] = stats{
				cpuPct: 0,
				memPct: 0,
				err:    err,
			}
			continue
		}
		(*results)[i] = stats{
			cpuPct: info.CPUPercent,
			memPct: info.MemoryPercent,
			err:    err,
		}
	}
}
