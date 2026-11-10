package experimentapi

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MaxBernstetter/dagobert/src/shared/providerurl"
)

// BackfillProviderBinance is the only ingestion provider with backfills (the legacy POST /backfill default too).
const BackfillProviderBinance = "binance"

const (
	backfillHourMS = int64(time.Hour / time.Millisecond)
	// backfillMaxRangeMS bounds one request: a year per market at most.
	backfillMaxRangeMS = 366 * 24 * backfillHourMS
)

// BackfillMarket is one allowed symbol with its base and quote asset.
type BackfillMarket struct {
	Symbol string
	Base   string
	Quote  string
}

// backfillMarkets are the symbols the lake accepts (the same set as the legacy POST /backfill).
var backfillMarkets = map[string]BackfillMarket{
	"BTCUSDT":  {Symbol: "BTCUSDT", Base: "BTC", Quote: "USDT"},
	"ETHUSDT":  {Symbol: "ETHUSDT", Base: "ETH", Quote: "USDT"},
	"USDCUSDT": {Symbol: "USDCUSDT", Base: "USDC", Quote: "USDT"},
}

// BackfillMarkets resolves the requested symbols in request order, without duplicates. Empty means BTCUSDT and ETHUSDT.
func BackfillMarkets(symbols []string) ([]BackfillMarket, error) {
	if len(symbols) == 0 {
		symbols = []string{"BTCUSDT", "ETHUSDT"}
	}
	seen := map[string]bool{}
	out := make([]BackfillMarket, 0, len(symbols))
	for _, s := range symbols {
		m, ok := backfillMarkets[s]
		if !ok {
			return nil, fmt.Errorf("unknown symbol %q", s)
		}
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, m)
	}
	return out, nil
}

// ResolveBackfillProvider returns the provider id. Empty means binance.
func ResolveBackfillProvider(provider string) (string, error) {
	p := strings.TrimSpace(provider)
	if p == "" {
		return BackfillProviderBinance, nil
	}
	if p != BackfillProviderBinance {
		return "", fmt.Errorf("unknown provider %q", p)
	}
	return p, nil
}

// BackfillWindow validates the requested window [fromMs, toMs) and returns the window the executions run:
// toMs is clamped to nowMs and rounded down to the hour, so only closed hours are backfilled.
func BackfillWindow(fromMs, toMs, nowMs int64) (int64, int64, error) {
	if fromMs <= 0 || toMs <= fromMs {
		return 0, 0, fmt.Errorf("from_ms and to_ms must be unix milliseconds with from_ms < to_ms")
	}
	if toMs > nowMs {
		toMs = nowMs
	}
	toMs -= toMs % backfillHourMS
	if fromMs >= toMs {
		return 0, 0, fmt.Errorf("the range ends inside the current open hour")
	}
	if toMs-fromMs > backfillMaxRangeMS {
		return 0, 0, fmt.Errorf("the range is longer than 366 days")
	}
	return fromMs, toMs, nil
}

// BackfillInput is the state machine input of one market (backfill_sfn.tf). BinanceBaseURL is applied to the
// ingest and indicator containers as BINANCE_BASE_URL. Empty means the SSM value of the job.
type BackfillInput struct {
	Provider       string `json:"provider"`
	Base           string `json:"base"`
	Quote          string `json:"quote"`
	From           int64  `json:"from"`
	To             int64  `json:"to"`
	BinanceBaseURL string `json:"binance_base_url,omitempty"`
}

// BackfillInputFor maps one market of a validated request to its execution input.
func BackfillInputFor(provider string, m BackfillMarket, from, to int64, savedConfig map[string]string, envBaseURL string) BackfillInput {
	return BackfillInput{
		Provider:       provider,
		Base:           m.Base,
		Quote:          m.Quote,
		From:           from,
		To:             to,
		BinanceBaseURL: providerurl.EffectiveBinance(savedConfig["baseUrl"], envBaseURL),
	}
}

// BackfillNamePrefix is shared by the executions of one provider market. Running ones are found by it.
func BackfillNamePrefix(provider, symbol string) string {
	return provider + "-" + symbol + "-"
}

// BackfillExecutionName is the unique execution name of one start (at most 80 characters).
func BackfillExecutionName(provider, symbol string, fromMs, toMs, startedMs int64) string {
	return BackfillNamePrefix(provider, symbol) + strconv.FormatInt(fromMs, 10) + "-" +
		strconv.FormatInt(toMs, 10) + "-" + strconv.FormatInt(startedMs, 10)
}

// BackfillRunState maps a Step Functions execution status and cause to the API status and error.
// RUNNING and SUCCEEDED map to themselves, TIMED_OUT reports as FAILED, and an error is set only for
// executions that did not succeed.
func BackfillRunState(status, cause string) (string, string) {
	switch status {
	case "RUNNING", "PENDING_REDRIVE":
		return "RUNNING", ""
	case "SUCCEEDED":
		return "SUCCEEDED", ""
	case "ABORTED":
		return "ABORTED", errorOrStatus(cause, status)
	case "FAILED", "TIMED_OUT":
		return "FAILED", errorOrStatus(cause, status)
	default:
		return status, errorOrStatus(cause, status)
	}
}

func errorOrStatus(cause, status string) string {
	if cause != "" {
		return cause
	}
	return "workflow " + strings.ToLower(status)
}
