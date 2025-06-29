package functions

import (
	"sync/atomic"
	"time"

	"github.com/xaydras-2/loadBalancer/App/config"
)

func AutoScaler() {
	ticker := time.NewTicker(config.ScaleInterval)
	defer ticker.Stop()

	// Every interval (15s in our case), check request volume and adjust replicas.
	for range ticker.C {
		// 1) Snapshot and reset request count atomically
		count := atomic.SwapInt64(&config.ReqCount, 0)

		config.BackendsMu.Lock()
		replicas := len(config.Backends)
		switch {
		// for every set time interval(15s for example) this check will be triggered
		// the check will see if the reqCount has passed scale up threshold req
		case count > int64(config.ScaleUpThreshold) && replicas < config.MaxReplicas:
			// scale up inline
			go ScaleUp()
		// same in here but it will check if reqCount is less than the scale down threshold
		case count < int64(config.ScaleDownThreshold) && replicas > config.MinReplicas:
			//scale down inline
			go ScaleDown()
		}

		config.BackendsMu.Unlock()
	}
}
