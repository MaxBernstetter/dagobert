package fetchclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MaxBernstetter/dagobert/src/ingest/provider"
	"github.com/MaxBernstetter/dagobert/src/shared"
)

func TestWindows(t *testing.T) {
	const h = hourMS
	missing := []int64{0, h, 2 * h, 5 * h} // gap between 2h and 5h
	got := Windows(missing, 10*h, 2)
	want := [][2]int64{
		{0, 2*h - 1},
		{2 * h, 3*h - 1},
		{5 * h, 6*h - 1},
	}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("win[%d]=%v want %v", i, got[i], want[i])
		}
	}
}

func TestFetchAll_retriesThenSucceeds(t *testing.T) {
	ad := &fakeAdapter{
		weight: 1,
		maxH:   1000,
		fn: func(n int32, req provider.PageRequest) provider.PageResult {
			if n == 1 {
				return provider.PageResult{
					StatusCode: http.StatusTooManyRequests,
					RetryAfter: -1,
					Err:        errors.New("rate limited"),
				}
			}
			return provider.PageResult{
				StatusCode: http.StatusOK,
				Candles: []provider.Candle{{
					OpenTime: req.Start, Open: "1", High: "1", Low: "1", Close: "1", Volume: "1",
					CloseTime: req.Start + hourMS - 1,
				}},
			}
		},
	}
	cfg := TransportConfig{
		MaxRetries: 2, BackoffFactor: 1, RetryWait: time.Millisecond,
		MaxRetryWait: time.Millisecond, RateLimit: 100, Concurrency: 1,
	}
	rows, err := FetchAll(context.Background(), ad, "BTC", "USDT", []int64{0}, hourMS, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].OpenTime != 0 {
		t.Fatalf("rows=%v", rows)
	}
	if ad.calls.Load() != 2 {
		t.Fatalf("calls=%d", ad.calls.Load())
	}
}

func TestFetchAll_nonRetryable(t *testing.T) {
	ad := &fakeAdapter{
		weight: 1,
		maxH:   1000,
		fn: func(_ int32, _ provider.PageRequest) provider.PageResult {
			return provider.PageResult{
				StatusCode: http.StatusBadRequest,
				RetryAfter: -1,
				Err:        errors.New("bad request"),
			}
		},
	}
	cfg := TransportConfig{
		MaxRetries: 5, BackoffFactor: 1, RetryWait: time.Millisecond,
		MaxRetryWait: time.Millisecond, RateLimit: 100, Concurrency: 1,
	}
	_, err := FetchAll(context.Background(), ad, "BTC", "USDT", []int64{0}, hourMS, cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if ad.calls.Load() != 1 {
		t.Fatalf("calls=%d want 1", ad.calls.Load())
	}
}

func TestFetchAll_honorsRetryAfter(t *testing.T) {
	ad := &fakeAdapter{
		weight: 1,
		maxH:   1000,
		fn: func(n int32, req provider.PageRequest) provider.PageResult {
			if n == 1 {
				return provider.PageResult{
					StatusCode: http.StatusTooManyRequests,
					RetryAfter: 20 * time.Millisecond,
					Err:        errors.New("rate limited"),
				}
			}
			return provider.PageResult{
				StatusCode: http.StatusOK,
				Candles:    []provider.Candle{{OpenTime: req.Start, Open: "1", High: "1", Low: "1", Close: "1", Volume: "1"}},
			}
		},
	}
	cfg := TransportConfig{
		MaxRetries: 2, BackoffFactor: 1, RetryWait: time.Hour, // would be huge without Retry-After
		MaxRetryWait: time.Hour, RateLimit: 100, Concurrency: 1,
	}
	start := time.Now()
	_, err := FetchAll(context.Background(), ad, "BTC", "USDT", []int64{0}, hourMS, cfg)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed < 15*time.Millisecond {
		t.Fatalf("elapsed %v; expected Retry-After wait", elapsed)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("elapsed %v; used backoff instead of Retry-After?", elapsed)
	}
}

func TestFetchAll_concurrency(t *testing.T) {
	var inflight atomic.Int32
	var maxInflight atomic.Int32
	ad := &fakeAdapter{
		weight: 1,
		maxH:   1, // one hour per window → many pages
		fn: func(_ int32, req provider.PageRequest) provider.PageResult {
			n := inflight.Add(1)
			for {
				cur := maxInflight.Load()
				if n <= cur || maxInflight.CompareAndSwap(cur, n) {
					break
				}
			}
			time.Sleep(30 * time.Millisecond)
			inflight.Add(-1)
			return provider.PageResult{
				StatusCode: http.StatusOK,
				Candles:    []provider.Candle{{OpenTime: req.Start, Open: "1", High: "1", Low: "1", Close: "1", Volume: "1"}},
			}
		},
	}
	missing := []int64{0, hourMS, 2 * hourMS, 3 * hourMS}
	cfg := TransportConfig{
		MaxRetries: 0, BackoffFactor: 1, RetryWait: time.Millisecond,
		MaxRetryWait: time.Millisecond, RateLimit: 10_000, Concurrency: 2,
	}
	rows, err := FetchAll(context.Background(), ad, "BTC", "USDT", missing, 4*hourMS, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("rows=%d", len(rows))
	}
	if maxInflight.Load() > 2 {
		t.Fatalf("max inflight=%d", maxInflight.Load())
	}
	if maxInflight.Load() < 2 {
		t.Fatalf("expected concurrency; max inflight=%d", maxInflight.Load())
	}
}

type fakeAdapter struct {
	weight int
	maxH   int
	calls  atomic.Int32
	fn     func(call int32, req provider.PageRequest) provider.PageResult
}

func (f *fakeAdapter) ID() string                         { return "fake" }
func (f *fakeAdapter) DataSource() shared.DataSource      { return shared.DataSourceBinance }
func (f *fakeAdapter) EmptyReason() string                { return "fake_empty" }
func (f *fakeAdapter) MaxPageHours() int                  { return f.maxH }
func (f *fakeAdapter) RequestWeight() int                 { return f.weight }
func (f *fakeAdapter) Retryable(status int, err error) bool {
	if status == http.StatusTooManyRequests || status == http.StatusTeapot || status >= 500 {
		return true
	}
	if status == 0 && err != nil {
		return true
	}
	return false
}
func (f *fakeAdapter) ListPairs(ctx context.Context) ([]provider.Pair, error) {
	return nil, nil
}
func (f *fakeAdapter) FetchPage(ctx context.Context, req provider.PageRequest) provider.PageResult {
	n := f.calls.Add(1)
	if f.fn == nil {
		return provider.PageResult{Err: fmt.Errorf("no fn")}
	}
	return f.fn(n, req)
}
