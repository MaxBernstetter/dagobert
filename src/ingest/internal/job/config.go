package job

import (
	"flag"
	"fmt"
	"io"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	hourMS             int64 = 3_600_000
	klineWeight              = 2
	pageLimit                = 1000
	defaultRetries           = 3
	defaultBackoff           = 2.0
	defaultRetryWait         = 200 * time.Millisecond
	defaultMaxWait           = 5 * time.Second
	defaultRateLimit         = 1200
	defaultConcurrency       = 2
)

var allowedSymbols = map[string]struct{}{
	"BTCUSDT":  {},
	"ETHUSDT":  {},
	"USDCUSDT": {},
}

// Config is one ingest run. Durations use Go's time.ParseDuration
// ("200ms", "5s"). Empty optional settings keep the defaults.
type Config struct {
	From          int64
	To            int64
	Symbol        string
	Force         bool
	DataDir       string
	BaseURL       string
	Endpoint      string
	MaxRetries    int
	BackoffFactor float64
	RetryWait     time.Duration
	MaxRetryWait  time.Duration
	RateLimit     int
	Concurrency   int
}

func parseConfig(args []string, getenv func(string) string) (Config, error) {
	fs := flag.NewFlagSet("ingest", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fromStr := fs.String("from", "", "first open time inclusive, ms UTC")
	toStr := fs.String("to", "", "last open time exclusive, ms UTC")
	symbol := fs.String("symbol", "", "BTCUSDT, ETHUSDT, or USDCUSDT")
	force := fs.Bool("force", false, "re-fetch all closed hours in range (replace parquet rows)")
	if err := fs.Parse(args); err != nil {
		return Config{}, fmt.Errorf("flags: %w", err)
	}
	if fs.NArg() != 0 || *fromStr == "" || *toStr == "" || *symbol == "" {
		return Config{}, fmt.Errorf("missing required flags")
	}
	from, err := strconv.ParseInt(*fromStr, 10, 64)
	if err != nil {
		return Config{}, fmt.Errorf("from: %w", err)
	}
	to, err := strconv.ParseInt(*toStr, 10, 64)
	if err != nil {
		return Config{}, fmt.Errorf("to: %w", err)
	}
	if from >= to {
		return Config{}, fmt.Errorf("from >= to")
	}
	if _, ok := allowedSymbols[*symbol]; !ok {
		return Config{}, fmt.Errorf("symbol")
	}

	dataDir := strings.TrimSpace(getenv("DATA_DIR"))
	baseURL := strings.TrimSpace(getenv("BINANCE_BASE_URL"))
	if dataDir == "" || baseURL == "" {
		return Config{}, fmt.Errorf("DATA_DIR and BINANCE_BASE_URL are required")
	}
	endpoint, err := klineEndpoint(baseURL)
	if err != nil {
		return Config{}, err
	}

	maxRetries, err := envInt(getenv, "max_retries", defaultRetries)
	if err != nil || maxRetries < 0 {
		return Config{}, fmt.Errorf("max_retries")
	}
	factor, err := envFloat(getenv, "backoff_factor", defaultBackoff)
	if err != nil || factor < 0 || math.IsNaN(factor) || math.IsInf(factor, 0) {
		return Config{}, fmt.Errorf("backoff_factor")
	}
	retryWait, err := envDuration(getenv, "retry_wait", defaultRetryWait)
	if err != nil || retryWait < 0 {
		return Config{}, fmt.Errorf("retry_wait")
	}
	maxWait, err := envDuration(getenv, "max_retry_wait", defaultMaxWait)
	if err != nil || maxWait < 0 {
		return Config{}, fmt.Errorf("max_retry_wait")
	}
	rate, err := envInt(getenv, "rate_limit", defaultRateLimit)
	if err != nil || rate < klineWeight {
		return Config{}, fmt.Errorf("rate_limit")
	}
	conc, err := envInt(getenv, "concurrency", defaultConcurrency)
	if err != nil || conc < 1 {
		return Config{}, fmt.Errorf("concurrency")
	}

	return Config{
		From:          from,
		To:            to,
		Symbol:        *symbol,
		Force:         *force,
		DataDir:       dataDir,
		BaseURL:       baseURL,
		Endpoint:      endpoint,
		MaxRetries:    maxRetries,
		BackoffFactor: factor,
		RetryWait:     retryWait,
		MaxRetryWait:  maxWait,
		RateLimit:     rate,
		Concurrency:   conc,
	}, nil
}

func klineEndpoint(base string) (string, error) {
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("BINANCE_BASE_URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("BINANCE_BASE_URL")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/v3/klines"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func envInt(getenv func(string) string, key string, def int) (int, error) {
	v := strings.TrimSpace(getenv(key))
	if v == "" {
		return def, nil
	}
	return strconv.Atoi(v)
}

func envFloat(getenv func(string) string, key string, def float64) (float64, error) {
	v := strings.TrimSpace(getenv(key))
	if v == "" {
		return def, nil
	}
	return strconv.ParseFloat(v, 64)
}

func envDuration(getenv func(string) string, key string, def time.Duration) (time.Duration, error) {
	v := strings.TrimSpace(getenv(key))
	if v == "" {
		return def, nil
	}
	return time.ParseDuration(v)
}
