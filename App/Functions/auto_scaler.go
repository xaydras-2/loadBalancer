// Package functions implements core logic for active monitoring, load balancing,
// and auto-scaling of back-end services.
package functions

import (
	"log"
	"sync/atomic"
	"time"

	"github.com/xaydras-2/loadBalancer/App/config"
)

// AutoScaler periodically checks the request volume and adjusts the number of
// replicas according to the configured thresholds. If the request count is
// greater than the scale up threshold and there are less than the maximum number
// of replicas, it starts a new replica. If the request count is less than the
// scale down threshold and there are more than the minimum number of replicas, it
// removes one replica. The check is done every ScaleInterval (default is 15s).
func AutoScaler() {
	ticker := time.NewTicker(config.ScaleInterval)
	defer ticker.Stop()

	log.Printf("i'm called AutoScaler")

	// Every interval (15s in our case), check request volume and adjust replicas.
	for range ticker.C {
		// 1) Snapshot and reset request count atomically
		count := atomic.SwapInt64(&config.ReqCount, 0)

		config.BackendsMu.Lock()
		replicas := config.Backends.Len()
		config.BackendsMu.Unlock()
		switch {
		// for every set time interval(15s for example) this check will be triggered
		// the check will see if the reqCount has passed scale up threshold req
		case count > int64(config.ScaleUpThreshold) && replicas < config.MaxReplicas:
			// scale up inline
			ScaleUp()
		// same in here but it will check if reqCount is less than the scale down threshold
		case count < int64(config.ScaleDownThreshold) && replicas > config.MinReplicas:
			//scale down inline
			ScaleDown()
		}

		
	}
}
