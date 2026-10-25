package experimentapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/jackc/pgx/v5/pgtype"
)

type normalizedConfig struct {
	Playbook       string
	StrategyConfig map[string]interface{}
	Hash           string
	LabelFrag      string
	EntryID        string
	CfgParams      experimentdb.InsertInstanceParams
}

func normalizePlaybook(playbook string) string {
	return strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(playbook), ".yaml"), ".yml")
}

func sha256JSON(v interface{}) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func normalizeMaCrossConfig(raw map[string]interface{}) (*v1.MaCrossConfig, error) {
	fast, ok1 := numberVal(raw["fastHr"])
	slow, ok2 := numberVal(raw["slowHr"])
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("ma-cross config: fastHr/slowHr required")
	}
	fi, si := int(fast), int(slow)
	if !allowedSMA[fi] || !allowedSMA[si] {
		return nil, fmt.Errorf("ma-cross config: fastHr/slowHr must be in {3,6,12,24,48}")
	}
	if fi >= si {
		return nil, fmt.Errorf("ma-cross config: fastHr must be < slowHr")
	}
	return &v1.MaCrossConfig{FastHr: int32(fi), SlowHr: int32(si)}, nil
}

func normalizeSignalConsensusConfig(raw map[string]interface{}) (*v1.SignalConsensusConfig, error) {
	cfg := &v1.SignalConsensusConfig{
		RsiBull:               pickFloat(raw, "rsiBull", 52),
		RsiBear:               pickFloat(raw, "rsiBear", 48),
		VolRatioBull:          pickFloat(raw, "volRatioBull", 1.1),
		VolRatioBear:          pickFloat(raw, "volRatioBear", 0.85),
		AtrStopMult:           pickFloat(raw, "atrStopMult", 2.5),
		MinBarsFlatAfterSell:  int32(pickFloat(raw, "minBarsFlatAfterSell", 8)),
		MinBarsLongBeforeSell: int32(pickFloat(raw, "minBarsLongBeforeSell", 4)),
	}
	if cfg.RsiBull <= cfg.RsiBear {
		return cfg, fmt.Errorf("signal-consensus config: rsiBull must be > rsiBear")
	}
	if cfg.VolRatioBull <= cfg.VolRatioBear || cfg.VolRatioBull <= 0 || cfg.VolRatioBear <= 0 {
		return cfg, fmt.Errorf("signal-consensus config: invalid volume ratios")
	}
	if cfg.AtrStopMult <= 0 {
		return cfg, fmt.Errorf("signal-consensus config: atrStopMult must be positive")
	}
	if cfg.MinBarsFlatAfterSell < 0 || cfg.MinBarsLongBeforeSell < 0 {
		return cfg, fmt.Errorf("signal-consensus config: cooldowns must be >= 0")
	}
	return cfg, nil
}

func pickFloat(raw map[string]interface{}, key string, def float64) float64 {
	if v, ok := numberVal(raw[key]); ok {
		return v
	}
	return def
}

func strategyMapFromSpec(spec *v1.ExperimentInstanceSpec) map[string]interface{} {
	if spec == nil {
		return nil
	}
	if spec.MaCross != nil {
		return map[string]interface{}{
			"fastHr": spec.MaCross.FastHr,
			"slowHr": spec.MaCross.SlowHr,
		}
	}
	if spec.SignalConsensus != nil {
		c := spec.SignalConsensus
		return map[string]interface{}{
			"rsiBull":               c.RsiBull,
			"rsiBear":               c.RsiBear,
			"volRatioBull":          c.VolRatioBull,
			"volRatioBear":          c.VolRatioBear,
			"atrStopMult":           c.AtrStopMult,
			"minBarsFlatAfterSell":  c.MinBarsFlatAfterSell,
			"minBarsLongBeforeSell": c.MinBarsLongBeforeSell,
		}
	}
	return nil
}

func normalizeInstanceConfig(playbook string, raw map[string]interface{}) (normalizedConfig, error) {
	slug := normalizePlaybook(playbook)
	switch slug {
	case "ma-cross":
		cfg, err := normalizeMaCrossConfig(raw)
		if err != nil {
			return normalizedConfig{}, err
		}
		strategy := map[string]interface{}{"fastHr": cfg.FastHr, "slowHr": cfg.SlowHr}
		params := experimentdb.InsertInstanceParams{
			CfgFastHr: pgtype.Int4{Int32: cfg.FastHr, Valid: true},
			CfgSlowHr: pgtype.Int4{Int32: cfg.SlowHr, Valid: true},
		}
		return normalizedConfig{
			Playbook:       slug,
			StrategyConfig: strategy,
			Hash:           sha256JSON(map[string]int32{"fastHr": cfg.FastHr, "slowHr": cfg.SlowHr}),
			LabelFrag:      fmt.Sprintf("%dh / %dh", cfg.FastHr, cfg.SlowHr),
			EntryID:        "ma-cross.yaml",
			CfgParams:      params,
		}, nil
	case "signal-consensus":
		cfg, err := normalizeSignalConsensusConfig(raw)
		if err != nil {
			return normalizedConfig{}, err
		}
		strategy := map[string]interface{}{
			"rsiBull": cfg.RsiBull, "rsiBear": cfg.RsiBear,
			"volRatioBull": cfg.VolRatioBull, "volRatioBear": cfg.VolRatioBear,
			"atrStopMult":          cfg.AtrStopMult,
			"minBarsFlatAfterSell": cfg.MinBarsFlatAfterSell, "minBarsLongBeforeSell": cfg.MinBarsLongBeforeSell,
		}
		params := experimentdb.InsertInstanceParams{
			CfgRsiBull:               pgtype.Float8{Float64: cfg.RsiBull, Valid: true},
			CfgRsiBear:               pgtype.Float8{Float64: cfg.RsiBear, Valid: true},
			CfgVolRatioBull:          pgtype.Float8{Float64: cfg.VolRatioBull, Valid: true},
			CfgVolRatioBear:          pgtype.Float8{Float64: cfg.VolRatioBear, Valid: true},
			CfgAtrStopMult:           pgtype.Float8{Float64: cfg.AtrStopMult, Valid: true},
			CfgMinBarsFlatAfterSell:  pgtype.Int4{Int32: cfg.MinBarsFlatAfterSell, Valid: true},
			CfgMinBarsLongBeforeSell: pgtype.Int4{Int32: cfg.MinBarsLongBeforeSell, Valid: true},
		}
		return normalizedConfig{
			Playbook:       slug,
			StrategyConfig: strategy,
			Hash:           sha256JSON(cfg),
			LabelFrag:      fmt.Sprintf("rsi %.0f/%.0f atr×%.1f", cfg.RsiBull, cfg.RsiBear, cfg.AtrStopMult),
			EntryID:        "signal-consensus.yaml",
			CfgParams:      params,
		}, nil
	default:
		return normalizedConfig{}, fmt.Errorf("experiment instances do not support playbook %s", playbook)
	}
}

func mergeSettings(in *v1.ExperimentSettings) *v1.ExperimentSettings {
	out := defaultExperimentSettings()
	if in == nil {
		return out
	}
	if strings.TrimSpace(in.Provider) != "" {
		out.Provider = strings.TrimSpace(in.Provider)
	}
	if strings.TrimSpace(in.Base) != "" {
		out.Base = strings.ToUpper(strings.TrimSpace(in.Base))
	}
	if strings.TrimSpace(in.Quote) != "" {
		out.Quote = strings.ToUpper(strings.TrimSpace(in.Quote))
	}
	if in.FromMs > 0 {
		out.FromMs = in.FromMs
	}
	if in.ToMs > 0 {
		out.ToMs = in.ToMs
	}
	if in.InitialUsdc != 0 {
		out.InitialUsdc = in.InitialUsdc
	}
	if in.InitialUsdt != 0 {
		out.InitialUsdt = in.InitialUsdt
	}
	if in.InitialBase != 0 {
		out.InitialBase = in.InitialBase
	}
	if in.TargetWeightPct != 0 {
		out.TargetWeightPct = in.TargetWeightPct
	}
	if in.CashReserveQuote != 0 {
		out.CashReserveQuote = in.CashReserveQuote
	}
	if in.MinOrderQuote != 0 {
		out.MinOrderQuote = in.MinOrderQuote
	}
	out.RedownloadMissing = in.RedownloadMissing
	if strings.TrimSpace(in.Algo) != "" {
		out.Algo = in.Algo
	}
	if in.SpreadBps != 0 {
		out.SpreadBps = in.SpreadBps
	}
	if in.SlippageBps != 0 {
		out.SlippageBps = in.SlippageBps
	}
	if in.FeeBps != 0 {
		out.FeeBps = in.FeeBps
	}
	return out
}

func defaultExperimentSettings() *v1.ExperimentSettings {
	now := timeNowMs()
	to := now - (now % (60 * 60 * 1000))
	from := to - 365*24*60*60*1000
	return &v1.ExperimentSettings{
		Provider:          "binance",
		Base:              "BTC",
		Quote:             "USDT",
		FromMs:            from,
		ToMs:              to,
		InitialUsdc:       100_000,
		InitialUsdt:       0,
		InitialBase:       0,
		TargetWeightPct:   100,
		CashReserveQuote:  0,
		MinOrderQuote:     1,
		RedownloadMissing: false,
		Algo:              "v1",
		SpreadBps:         4,
		SlippageBps:       2,
		FeeBps:            6,
	}
}

func timeNowMs() int64 {
	return time.Now().UnixMilli()
}
