package httpclient

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
)

var (
	rateLimitMu    sync.RWMutex
	requestLimiter *rate.Limiter
	currentRPS     float64
	currentBurst   = 1
)

// SetRateLimit configures the shared process-wide request limiter.
// rps <= 0 disables throttling.
func SetRateLimit(rps float64, burst int) {
	rateLimitMu.Lock()
	defer rateLimitMu.Unlock()
	if burst < 1 {
		burst = 1
	}
	currentRPS = rps
	currentBurst = burst
	if rps <= 0 {
		requestLimiter = nil
		return
	}
	requestLimiter = rate.NewLimiter(rate.Limit(rps), burst)
}

// CurrentRateLimit returns the active limiter settings.
func CurrentRateLimit() (float64, int) {
	rateLimitMu.RLock()
	defer rateLimitMu.RUnlock()
	return currentRPS, currentBurst
}

func waitRateLimit(ctx context.Context) error {
	rateLimitMu.RLock()
	limiter := requestLimiter
	rateLimitMu.RUnlock()
	if limiter == nil {
		return nil
	}
	return limiter.Wait(ctx)
}
