// Package providerurl resolves the base URL an ingestion provider reads from. The backfill job containers
// and the Backfill API (backfill executions) use the same precedence.
package providerurl

import "strings"

// DefaultBinanceBaseURL is the public Binance data host used when nothing else is set.
const DefaultBinanceBaseURL = "https://data-api.binance.vision"

// EffectiveBinance is the base URL the Binance provider reads from: the saved config baseUrl when set, else the
// BINANCE_BASE_URL environment value, else the public default.
func EffectiveBinance(saved, envValue string) string {
	if v := strings.TrimSpace(saved); v != "" {
		return v
	}
	if v := strings.TrimSpace(envValue); v != "" {
		return v
	}
	return DefaultBinanceBaseURL
}
