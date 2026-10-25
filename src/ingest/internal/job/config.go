package job

import (
	"flag"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/MaxBernstetter/dagobert/src/ingest/provider"
	"github.com/MaxBernstetter/dagobert/src/shared"
)

const (
	hourMS             int64 = 3_600_000
	defaultRetries           = 3
	defaultBackoff           = 2.0
	defaultRetryWait         = 200 * time.Millisecond
	defaultMaxWait           = 5 * time.Second
	defaultRateLimit         = 1200
	defaultConcurrency       = 2
)

// Config is one ingest run. Durations use Go's time.ParseDuration
// ("200ms", "5s"). Empty optional settings keep the defaults.
type Config struct {
	From          int64
	To            int64
	Base          string
	Quote         string
	Provider      string
	Force         bool
	DataDir       string
	BaseURL       string
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
	baseFlag := fs.String("base", "", "base currency code, e.g. BTC")
	quoteFlag := fs.String("quote", "", "quote currency code, e.g. USDT")
	// Legacy -symbol BTCUSDT still accepted and split for Binance-style concat ids.
	symbolLegacy := fs.String("symbol", "", "deprecated: use -base and -quote")
	providerFlag := fs.String("provider", "", "market data provider id (default binance)")
	force := fs.Bool("force", false, "re-fetch all closed hours in range (replace parquet rows)")
	if err := fs.Parse(args); err != nil {
		return Config{}, fmt.Errorf("flags: %w", err)
	}
	if fs.NArg() != 0 || *fromStr == "" || *toStr == "" {
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

	base := strings.TrimSpace(*baseFlag)
	quote := strings.TrimSpace(*quoteFlag)
	if base == "" || quote == "" {
		if leg := strings.TrimSpace(*symbolLegacy); leg != "" {
			b, q, ok := splitLegacySymbol(leg)
			if !ok {
				return Config{}, fmt.Errorf("symbol")
			}
			base, quote = b, q
		}
	}
	m, err := shared.NormalizeMarket(base, quote)
	if err != nil {
		return Config{}, err
	}

	prov := strings.TrimSpace(*providerFlag)
	if prov == "" {
		prov = strings.TrimSpace(getenv("INGEST_PROVIDER"))
	}
	if prov == "" {
		prov = provider.DefaultID()
	}
	if err := shared.ValidatePartitionValue(prov); err != nil {
		return Config{}, fmt.Errorf("provider: %w", err)
	}

	dataDir := strings.TrimSpace(getenv("DATA_DIR"))
	if dataDir == "" {
		return Config{}, fmt.Errorf("DATA_DIR is required")
	}

	baseURL := strings.TrimSpace(getenv("BINANCE_BASE_URL"))
	if prov == shared.DataSourceBinance.String() && baseURL == "" {
		return Config{}, fmt.Errorf("BINANCE_BASE_URL is required for provider binance")
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
	if err != nil || rate < 1 {
		return Config{}, fmt.Errorf("rate_limit")
	}
	conc, err := envInt(getenv, "concurrency", defaultConcurrency)
	if err != nil || conc < 1 {
		return Config{}, fmt.Errorf("concurrency")
	}

	return Config{
		From:          from,
		To:            to,
		Base:          m.Base,
		Quote:         m.Quote,
		Provider:      prov,
		Force:         *force,
		DataDir:       dataDir,
		BaseURL:       baseURL,
		MaxRetries:    maxRetries,
		BackoffFactor: factor,
		RetryWait:     retryWait,
		MaxRetryWait:  maxWait,
		RateLimit:     rate,
		Concurrency:   conc,
	}, nil
}

// splitLegacySymbol maps common quote suffixes for old -symbol flags.
func splitLegacySymbol(sym string) (base, quote string, ok bool) {
	sym = strings.ToUpper(strings.TrimSpace(sym))
	quotes := []string{"USDT", "USDC", "BUSD", "FDUSD", "TUSD", "BTC", "ETH", "BNB", "EUR", "TRY", "BRL", "GBP", "AUD", "USD"}
	for _, q := range quotes {
		if strings.HasSuffix(sym, q) && len(sym) > len(q) {
			return sym[:len(sym)-len(q)], q, true
		}
	}
	return "", "", false
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
