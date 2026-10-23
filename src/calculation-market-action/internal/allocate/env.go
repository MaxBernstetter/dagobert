package allocate

import (
	"os"
	"strconv"
)

// PolicyFromEnv merges DefaultPolicy with ALLOC_* environment variables.
func PolicyFromEnv() Policy {
	p := DefaultPolicy()
	if v := os.Getenv("ALLOC_TARGET_WEIGHT_PCT"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			p.TargetWeightPct = f
		}
	}
	if v := os.Getenv("ALLOC_CASH_RESERVE_QUOTE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			p.ReserveQuote = f
		}
	}
	if v := os.Getenv("ALLOC_MIN_ORDER_QUOTE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			p.MinOrderQuote = f
		}
	}
	return p
}
