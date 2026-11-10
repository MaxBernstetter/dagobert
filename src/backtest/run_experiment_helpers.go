package backtest

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	ingestrunner "github.com/MaxBernstetter/dagobert/src/ingest/runner"
	"github.com/MaxBernstetter/dagobert/src/internal/artifacts"
	"github.com/MaxBernstetter/dagobert/src/internal/instruments"
	"github.com/MaxBernstetter/dagobert/src/internal/money"
	"github.com/MaxBernstetter/dagobert/src/shared"
)

// DefaultExperimentSimWorkers is used when EXPERIMENT_SIM_WORKERS is unset or invalid: one KPI replay
// worker per CPU. The replay is CPU bound Go work that runs beside the C++ sweep processes.
var DefaultExperimentSimWorkers = runtime.NumCPU()

// ExperimentSimWorkers returns the KPI replay worker count from EXPERIMENT_SIM_WORKERS (default: the CPU
// count, min 1).
func ExperimentSimWorkers() int {
	raw := strings.TrimSpace(os.Getenv("EXPERIMENT_SIM_WORKERS"))
	if raw == "" {
		return DefaultExperimentSimWorkers
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return DefaultExperimentSimWorkers
	}
	return n
}

func experimentSymbols(instances []store.ExperimentInstance, settings store.ExperimentSettings) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, inst := range instances {
		sym := store.InstanceSymbol(inst, settings)
		if _, ok := seen[sym]; ok {
			continue
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
	}
	return out
}

// paperVenueCoins puts the whole starting book in the venue quote the allocator can spend.
// For BTCUSDT and ETHUSDT that quote is USDC, so a split USDC/USDT balance is not left untradable.
func paperVenueCoins(symbol string, book, initialBase float64) map[string]float64 {
	base, quote := "BTC", "USDC"
	if inst, ok := instruments.Resolve(symbol); ok {
		if inst.Base != "" {
			base = inst.Base
		}
		if inst.Quote != "" {
			quote = inst.Quote
		}
	} else if b, q, ok := shared.SplitPipelineSymbol(symbol); ok {
		base, quote = b, q
	}
	coins := map[string]float64{quote: book}
	if base != quote {
		coins[base] = initialBase
	}
	return coins
}

// experimentWalletCoins is the starting paper wallet of an experiment instance. It depends only on
// the experiment settings and the instance symbol, so simulate derives it instead of reading a
// per-instance file (3000 instances would be 3000 objects).
func experimentWalletCoins(s store.ExperimentSettings, symbol string) map[string]float64 {
	coins := paperVenueCoins(symbol, s.InitialUSDC+s.InitialUSDT, s.InitialBase)
	for cur, amt := range coins {
		coins[cur] = money.RoundCoin(cur, amt)
	}
	return coins
}

const ingestChunkMS = int64(30 * 24 * 60 * 60 * 1000)

func ingestExperimentRange(provider, base, quote string, from, to int64, force bool) error {
	for start := from; start < to; start += ingestChunkMS {
		end := start + ingestChunkMS
		if end > to {
			end = to
		}
		args := []string{
			"-from", fmt.Sprint(start),
			"-to", fmt.Sprint(end),
			"-base", base,
			"-quote", quote,
			"-provider", provider,
		}
		if force {
			args = append(args, "-force")
		}
		code, detail := ingestrunner.Run(args, os.Getenv)
		if code != 0 && code != 2 {
			return fmt.Errorf(
				"ingest failed [%s, %s) exit=%d detail=%d (%s)",
				formatUTC(start), formatUTC(end), code, detail, ingestDetailHint(detail),
			)
		}
	}
	return nil
}

func ingestDetailHint(detail int) string {
	switch detail {
	case 10:
		return "invalid ingest input"
	case 30:
		return "Binance fetch failed or empty after retries — check network/rate limits; re-run to resume from parquet"
	case 40:
		return "lake/sqlite inconsistency"
	default:
		return "see ingest job logs"
	}
}

func applyExperimentEnv(dataDir string, s store.ExperimentSettings) {
	_ = os.Setenv("DATA_DIR", dataDir)
	_ = os.Unsetenv("AGGREGATION_DURATION_MS")
	if os.Getenv("BINANCE_BASE_URL") == "" {
		_ = os.Setenv("BINANCE_BASE_URL", "https://data-api.binance.vision")
	}
	_ = os.Setenv("ALLOC_TARGET_WEIGHT_PCT", strconv.FormatFloat(s.TargetWeightPct, 'f', -1, 64))
	_ = os.Setenv("ALLOC_CASH_RESERVE_QUOTE", strconv.FormatFloat(s.CashReserveQuote, 'f', -1, 64))
	_ = os.Setenv("ALLOC_MIN_ORDER_QUOTE", strconv.FormatFloat(s.MinOrderQuote, 'f', -1, 64))
	_ = os.Setenv("SIM_SPREAD_BPS", strconv.FormatFloat(s.SpreadBps, 'f', -1, 64))
	_ = os.Setenv("SIM_SLIPPAGE_BPS", strconv.FormatFloat(s.SlippageBps, 'f', -1, 64))
	_ = os.Setenv("SIM_FEE_BPS", strconv.FormatFloat(s.FeeBps, 'f', -1, 64))
	if os.Getenv("INGEST_MAX_RETRIES") == "" {
		_ = os.Setenv("INGEST_MAX_RETRIES", "12")
	}
	if os.Getenv("INGEST_MAX_RETRY_WAIT") == "" {
		_ = os.Setenv("INGEST_MAX_RETRY_WAIT", "120s")
	}
	_ = shared.EnsureDataLayout(dataDir)
}

// uploadInstanceReport dual-writes the report JSON to S3 when EXPERIMENT_ARTIFACTS=s3, under the
// instance's workspace_rel_path.
func uploadInstanceReport(workspaceRelPath string, body []byte) error {
	if !artifacts.Enabled() {
		return nil
	}
	return artifacts.PutBytes(context.Background(), artifacts.ObjectKey(workspaceRelPath), body)
}
