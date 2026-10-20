package config

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultBaseURL    = "https://api-demo.bybit.eu"
	DefaultRetries    = 3
	DefaultBackoff    = 2.0
	DefaultRetryWait  = 200 * time.Millisecond
	DefaultMaxWait    = 5 * time.Second
	RecvWindow        = 5000
)

type Retry struct {
	MaxRetries    int
	BackoffFactor float64
	RetryWait     time.Duration
	MaxRetryWait  time.Duration
}

func RetryFromEnv() (Retry, error) {
	maxRetries, err := envInt("MAX_RETRIES", DefaultRetries)
	if err != nil || maxRetries < 0 {
		return Retry{}, fmt.Errorf("MAX_RETRIES")
	}
	factor, err := envFloat("BACKOFF_FACTOR", DefaultBackoff)
	if err != nil || factor < 0 || math.IsNaN(factor) || math.IsInf(factor, 0) {
		return Retry{}, fmt.Errorf("BACKOFF_FACTOR")
	}
	retryWait, err := envDuration("RETRY_WAIT", DefaultRetryWait)
	if err != nil || retryWait < 0 {
		return Retry{}, fmt.Errorf("RETRY_WAIT")
	}
	maxWait, err := envDuration("MAX_RETRY_WAIT", DefaultMaxWait)
	if err != nil || maxWait < 0 {
		return Retry{}, fmt.Errorf("MAX_RETRY_WAIT")
	}
	return Retry{
		MaxRetries:    maxRetries,
		BackoffFactor: factor,
		RetryWait:     retryWait,
		MaxRetryWait:  maxWait,
	}, nil
}

func BaseURL() string {
	v := strings.TrimSpace(os.Getenv("BYBIT_BASE_URL"))
	if v == "" {
		return DefaultBaseURL
	}
	return strings.TrimRight(v, "/")
}

func envInt(key string, def int) (int, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def, nil
	}
	return strconv.Atoi(v)
}

func envFloat(key string, def float64) (float64, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def, nil
	}
	return strconv.ParseFloat(v, 64)
}

func envDuration(key string, def time.Duration) (time.Duration, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def, nil
	}
	return time.ParseDuration(v)
}
