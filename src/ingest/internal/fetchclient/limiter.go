package fetchclient

import (
	"context"
	"sync"
	"time"
)

type weightLimiter struct {
	mu     sync.Mutex
	tokens float64
	max    float64
	last   time.Time
}

func newWeightLimiter(perMinute int) *weightLimiter {
	return &weightLimiter{
		tokens: float64(perMinute),
		max:    float64(perMinute),
		last:   time.Now(),
	}
}

func (l *weightLimiter) acquire(ctx context.Context, weight float64) error {
	for {
		l.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(l.last).Seconds()
		l.tokens += elapsed * l.max / 60
		if l.tokens > l.max {
			l.tokens = l.max
		}
		l.last = now
		if l.tokens >= weight {
			l.tokens -= weight
			l.mu.Unlock()
			return nil
		}
		deficit := weight - l.tokens
		wait := time.Duration(deficit / l.max * float64(time.Minute))
		if wait < time.Millisecond {
			wait = time.Millisecond
		}
		l.mu.Unlock()
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
