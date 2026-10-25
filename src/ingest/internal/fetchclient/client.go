package fetchclient

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/MaxBernstetter/dagobert/src/ingest/provider"
)

const hourMS int64 = 3_600_000

// TransportConfig is shared HTTP fetch policy (env-backed from the job).
type TransportConfig struct {
	MaxRetries    int
	BackoffFactor float64
	RetryWait     time.Duration
	MaxRetryWait  time.Duration
	RateLimit     int // request weight per minute
	Concurrency   int
}

// FetchAll splits missing open times into pages and fetches them with shared transport.
func FetchAll(ctx context.Context, ad provider.Adapter, base, quote string, missing []int64, to int64, cfg TransportConfig) ([]provider.Candle, error) {
	if err := validate(cfg, ad); err != nil {
		return nil, err
	}
	wins := Windows(missing, to, ad.MaxPageHours())
	if len(wins) == 0 {
		return nil, nil
	}

	lim := newWeightLimiter(cfg.RateLimit)
	sem := make(chan struct{}, cfg.Concurrency)
	weight := float64(ad.RequestWeight())

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		mu    sync.Mutex
		all   []provider.Candle
		first error
		wg    sync.WaitGroup
	)
	for _, w := range wins {
		wg.Add(1)
		go func(start, end int64) {
			defer wg.Done()
			rows, err := fetchWindow(ctx, ad, base, quote, start, end, cfg, lim, sem, weight)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if first == nil && !errors.Is(err, context.Canceled) {
					first = err
					cancel()
				}
				return
			}
			all = append(all, rows...)
		}(w[0], w[1])
	}
	wg.Wait()
	if first != nil {
		return nil, first
	}
	return all, nil
}

func validate(cfg TransportConfig, ad provider.Adapter) error {
	if cfg.Concurrency < 1 {
		return fmt.Errorf("concurrency")
	}
	if cfg.MaxRetries < 0 {
		return fmt.Errorf("max_retries")
	}
	if cfg.RateLimit < ad.RequestWeight() {
		return fmt.Errorf("rate_limit")
	}
	if ad.MaxPageHours() < 1 {
		return fmt.Errorf("max_page_hours")
	}
	return nil
}

func fetchWindow(
	ctx context.Context,
	ad provider.Adapter,
	base, quote string,
	start, end int64,
	cfg TransportConfig,
	lim *weightLimiter,
	sem chan struct{},
	weight float64,
) ([]provider.Candle, error) {
	var wait time.Duration
	for try := 0; ; try++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
		if err := acquire(ctx, sem, lim, weight); err != nil {
			return nil, err
		}
		res := ad.FetchPage(ctx, provider.PageRequest{
			Base:  base,
			Quote: quote,
			Start: start,
			End:   end,
		})
		release(sem)
		if res.Err == nil {
			return res.Candles, nil
		}
		if errors.Is(res.Err, context.Canceled) {
			return nil, res.Err
		}
		if !ad.Retryable(res.StatusCode, res.Err) || try >= cfg.MaxRetries {
			return nil, res.Err
		}
		if res.RetryAfter >= 0 && res.StatusCode != 0 {
			wait = res.RetryAfter
		} else {
			wait = backoff(cfg, try)
		}
	}
}

func acquire(ctx context.Context, sem chan struct{}, lim *weightLimiter, weight float64) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case sem <- struct{}{}:
	}
	if err := lim.acquire(ctx, weight); err != nil {
		release(sem)
		return err
	}
	return nil
}

func release(sem chan struct{}) {
	<-sem
}

func backoff(cfg TransportConfig, failure int) time.Duration {
	pow := math.Pow(cfg.BackoffFactor, float64(failure))
	if math.IsNaN(pow) || math.IsInf(pow, 0) || pow < 0 {
		return cfg.MaxRetryWait
	}
	wait := time.Duration(float64(cfg.RetryWait) * pow)
	if wait < 0 || wait > cfg.MaxRetryWait {
		return cfg.MaxRetryWait
	}
	return wait
}

// Windows splits missing open times into contiguous pages of at most maxPageHours.
// The second value is the inclusive endTime: min(lastOpen+hour-1, to-1).
func Windows(missing []int64, to int64, maxPageHours int) [][2]int64 {
	if maxPageHours < 1 {
		return nil
	}
	var out [][2]int64
	for i := 0; i < len(missing); {
		j := i
		n := 1
		for j+1 < len(missing) && missing[j+1] == missing[j]+hourMS && n < maxPageHours {
			j++
			n++
		}
		start := missing[i]
		end := int64(math.MaxInt64)
		if missing[j] <= math.MaxInt64-hourMS+1 {
			end = missing[j] + hourMS - 1
		}
		if to-1 < end {
			end = to - 1
		}
		out = append(out, [2]int64{start, end})
		i = j + 1
	}
	return out
}
