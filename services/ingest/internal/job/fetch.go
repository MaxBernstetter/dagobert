package job

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type fetcher struct {
	endpoint      string
	symbol        string
	host          string
	http          *http.Client
	maxRetries    int
	backoffFactor float64
	retryWait     time.Duration
	maxRetryWait  time.Duration
	lim           *weightLimiter
	sem           chan struct{}
}

func newFetcher(cfg Config) (*fetcher, error) {
	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	f := &fetcher{
		endpoint:      cfg.Endpoint,
		symbol:        cfg.Symbol,
		host:          u.Host,
		maxRetries:    cfg.MaxRetries,
		backoffFactor: cfg.BackoffFactor,
		retryWait:     cfg.RetryWait,
		maxRetryWait:  cfg.MaxRetryWait,
		lim:           newWeightLimiter(cfg.RateLimit),
		sem:           make(chan struct{}, cfg.Concurrency),
	}
	f.http = &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 || req.URL.Host != f.host {
				return errors.New("refusing redirect")
			}
			return nil
		},
	}
	return f, nil
}

func (f *fetcher) fetch(ctx context.Context, wins [][2]int64) ([]Candle, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		mu    sync.Mutex
		all   []Candle
		first error
		wg    sync.WaitGroup
	)
	for _, w := range wins {
		wg.Add(1)
		go func(start, end int64) {
			defer wg.Done()
			rows, err := f.fetchWindow(ctx, start, end)
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

func (f *fetcher) fetchWindow(ctx context.Context, start, end int64) ([]Candle, error) {
	var all []Candle
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rows, err := f.getPage(ctx, start, end)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return all, nil
		}
		all = append(all, rows...)
		if len(rows) < pageLimit {
			return all, nil
		}
		next := rows[len(rows)-1].OpenTime + 1
		if next <= start || next > end {
			return all, nil
		}
		start = next
	}
}

func (f *fetcher) getPage(ctx context.Context, start, end int64) ([]Candle, error) {
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
		if err := f.acquire(ctx); err != nil {
			return nil, err
		}
		rows, status, retryAfter, err := f.do(ctx, start, end)
		f.release()
		if err == nil {
			return rows, nil
		}
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		if !retryable(status, err) || try >= f.maxRetries {
			return nil, err
		}
		if retryAfter >= 0 && status != 0 {
			wait = retryAfter
		} else {
			wait = f.backoff(try)
		}
	}
}

func retryable(status int, err error) bool {
	if status == http.StatusTooManyRequests || status == http.StatusTeapot || status >= 500 {
		return true
	}
	if status == 0 && err != nil {
		return true
	}
	if status == http.StatusOK && err != nil {
		return true
	}
	return false
}

func (f *fetcher) acquire(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case f.sem <- struct{}{}:
	}
	if err := f.lim.acquire(ctx, klineWeight); err != nil {
		f.release()
		return err
	}
	return nil
}

func (f *fetcher) release() {
	<-f.sem
}

func (f *fetcher) backoff(failure int) time.Duration {
	pow := math.Pow(f.backoffFactor, float64(failure))
	if math.IsNaN(pow) || math.IsInf(pow, 0) || pow < 0 {
		return f.maxRetryWait
	}
	wait := time.Duration(float64(f.retryWait) * pow)
	if wait < 0 || wait > f.maxRetryWait {
		return f.maxRetryWait
	}
	return wait
}

func (f *fetcher) do(ctx context.Context, start, end int64) ([]Candle, int, time.Duration, error) {
	u, err := url.Parse(f.endpoint)
	if err != nil {
		return nil, 0, 0, err
	}
	q := u.Query()
	q.Set("symbol", f.symbol)
	q.Set("interval", "1h")
	q.Set("startTime", strconv.FormatInt(start, 10))
	q.Set("endTime", strconv.FormatInt(end, 10))
	q.Set("limit", strconv.Itoa(pageLimit))
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, 0, err
	}
	resp, err := f.http.Do(req)
	if err != nil {
		return nil, 0, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, resp.StatusCode, 0, err
	}
	retryAfter, hasRA := parseRetryAfter(resp.Header.Get("Retry-After"))
	if resp.StatusCode != http.StatusOK {
		ra := time.Duration(-1)
		if hasRA {
			ra = retryAfter
		}
		return nil, resp.StatusCode, ra, fmt.Errorf("binance status %d", resp.StatusCode)
	}
	rows, err := parseKlines(body)
	if err != nil {
		return nil, resp.StatusCode, -1, err
	}
	return rows, resp.StatusCode, 0, nil
}

func parseRetryAfter(v string) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	if sec, err := strconv.Atoi(v); err == nil {
		if sec < 0 {
			return 0, true
		}
		return time.Duration(sec) * time.Second, true
	}
	when, err := http.ParseTime(v)
	if err != nil {
		return 0, false
	}
	d := time.Until(when)
	if d < 0 {
		return 0, true
	}
	return d, true
}

func parseKlines(body []byte) ([]Candle, error) {
	var raw [][]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := make([]Candle, 0, len(raw))
	for _, row := range raw {
		if len(row) < 7 {
			return nil, fmt.Errorf("short kline")
		}
		ot, err := rawInt(row[0])
		if err != nil {
			return nil, err
		}
		open, err := rawDec(row[1])
		if err != nil {
			return nil, err
		}
		high, err := rawDec(row[2])
		if err != nil {
			return nil, err
		}
		low, err := rawDec(row[3])
		if err != nil {
			return nil, err
		}
		cls, err := rawDec(row[4])
		if err != nil {
			return nil, err
		}
		vol, err := rawDec(row[5])
		if err != nil {
			return nil, err
		}
		ct, err := rawInt(row[6])
		if err != nil {
			return nil, err
		}
		out = append(out, Candle{
			OpenTime:  ot,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     cls,
			Volume:    vol,
			CloseTime: ct,
		})
	}
	return out, nil
}

func rawInt(m json.RawMessage) (int64, error) {
	dec := json.NewDecoder(bytes.NewReader(m))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return 0, err
	}
	switch n := v.(type) {
	case json.Number:
		return n.Int64()
	case string:
		return strconv.ParseInt(n, 10, 64)
	default:
		return 0, fmt.Errorf("bad int")
	}
}

func rawDec(m json.RawMessage) (string, error) {
	dec := json.NewDecoder(bytes.NewReader(m))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return "", err
	}
	switch n := v.(type) {
	case string:
		if n == "" {
			return "", fmt.Errorf("empty decimal")
		}
		return n, nil
	case json.Number:
		return n.String(), nil
	default:
		return "", fmt.Errorf("bad decimal")
	}
}

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
