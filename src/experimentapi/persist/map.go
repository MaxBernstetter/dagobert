// Package persist maps sqlc rows ↔ tradingcopilot.v1 messages.
// Call sites must only expose proto types.
package persist

import (
	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/jackc/pgx/v5/pgtype"
)

// ParallelismFromDB is the stored process count; NULL (auto) is nil, so the field is omitted.
func ParallelismFromDB(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	x := v.Int32
	return &x
}

// ParallelismToDB stores a positive process count; nil and 0 (auto) are stored as NULL.
func ParallelismToDB(v *int32) pgtype.Int4 {
	if v != nil && *v > 0 {
		return pgtype.Int4{Int32: *v, Valid: true}
	}
	return pgtype.Int4{}
}

// ParallelismValue is the stored count as a plain int; NULL is 0 (auto).
func ParallelismValue(v pgtype.Int4) int {
	if v.Valid {
		return int(v.Int32)
	}
	return 0
}

func optInt64(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	x := v.Int64
	return &x
}

func optInt32(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	x := v.Int32
	return &x
}

func optFloat32(v pgtype.Float4) *float32 {
	if !v.Valid {
		return nil
	}
	x := v.Float32
	return &x
}

func optFloat64(v pgtype.Float8) *float64 {
	if !v.Valid {
		return nil
	}
	x := v.Float64
	return &x
}

// ExperimentFromDB maps a experiment row (+ optional list enrichment) to proto.
func ExperimentFromDB(s experimentdb.Experiment, listPlaybooks, listSymbols []string) *v1.Experiment {
	out := &v1.Experiment{
		Id:                    s.ID.String(),
		Name:                  s.Name,
		Description:           s.Description,
		Status:                s.Status,
		Error:                 s.Error,
		Provider:              s.Provider,
		Base:                  s.Base,
		Quote:                 s.Quote,
		FromMs:                s.FromMs,
		ToMs:                  s.ToMs,
		InitialUsdc:           s.InitialUsdc,
		InitialUsdt:           s.InitialUsdt,
		InitialBase:           s.InitialBase,
		TargetWeightPct:       s.TargetWeightPct,
		CashReserveQuote:      s.CashReserveQuote,
		MinOrderQuote:         s.MinOrderQuote,
		RedownloadMissing:     s.RedownloadMissing,
		Algo:                  s.Algo,
		SpreadBps:             s.SpreadBps,
		SlippageBps:           s.SlippageBps,
		FeeBps:                s.FeeBps,
		Parallelism:           ParallelismFromDB(s.Parallelism),
		CreatedAtMs:           s.CreatedAtMs,
		StartedAtMs:           optInt64(s.StartedAtMs),
		FinishedAtMs:          optInt64(s.FinishedAtMs),
		ProgressStage:         s.ProgressStage,
		ProgressStages:        s.ProgressStages,
		ProgressStageLabel:    s.ProgressStageLabel,
		ProgressInstanceDone:  s.ProgressInstanceDone,
		ProgressInstanceTotal: s.ProgressInstanceTotal,
		ListPlaybooks:         listPlaybooks,
		ListSymbols:           listSymbols,
	}
	out.Settings = SettingsFromExperiment(out)
	return out
}

// SettingsFromExperiment extracts ExperimentSettings from a Experiment row message.
func SettingsFromExperiment(s *v1.Experiment) *v1.ExperimentSettings {
	if s == nil {
		return nil
	}
	return &v1.ExperimentSettings{
		Provider:          s.Provider,
		Base:              s.Base,
		Quote:             s.Quote,
		FromMs:            s.FromMs,
		ToMs:              s.ToMs,
		InitialUsdc:       s.InitialUsdc,
		InitialUsdt:       s.InitialUsdt,
		InitialBase:       s.InitialBase,
		TargetWeightPct:   s.TargetWeightPct,
		CashReserveQuote:  s.CashReserveQuote,
		MinOrderQuote:     s.MinOrderQuote,
		RedownloadMissing: s.RedownloadMissing,
		Algo:              s.Algo,
		SpreadBps:         s.SpreadBps,
		SlippageBps:       s.SlippageBps,
		FeeBps:            s.FeeBps,
		Parallelism:       s.Parallelism,
	}
}

// ApplySettings copies settings fields onto a Experiment message.
func ApplySettings(dst *v1.Experiment, settings *v1.ExperimentSettings) {
	if dst == nil || settings == nil {
		return
	}
	dst.Provider = settings.Provider
	dst.Base = settings.Base
	dst.Quote = settings.Quote
	dst.FromMs = settings.FromMs
	dst.ToMs = settings.ToMs
	dst.InitialUsdc = settings.InitialUsdc
	dst.InitialUsdt = settings.InitialUsdt
	dst.InitialBase = settings.InitialBase
	dst.TargetWeightPct = settings.TargetWeightPct
	dst.CashReserveQuote = settings.CashReserveQuote
	dst.MinOrderQuote = settings.MinOrderQuote
	dst.RedownloadMissing = settings.RedownloadMissing
	dst.Algo = settings.Algo
	dst.SpreadBps = settings.SpreadBps
	dst.SlippageBps = settings.SlippageBps
	dst.FeeBps = settings.FeeBps
	dst.Parallelism = settings.Parallelism
}

// ProgressFromDB maps experiment progress.
func ProgressFromDB(s experimentdb.Experiment, failedCount int64) *v1.ExperimentProgress {
	return &v1.ExperimentProgress{
		Id:                    s.ID.String(),
		Status:                s.Status,
		Error:                 s.Error,
		ProgressStage:         s.ProgressStage,
		ProgressStages:        s.ProgressStages,
		ProgressStageLabel:    s.ProgressStageLabel,
		ProgressInstanceDone:  s.ProgressInstanceDone,
		ProgressInstanceTotal: s.ProgressInstanceTotal,
		FailedInstanceCount:   failedCount,
		StartedAtMs:           optInt64(s.StartedAtMs),
		FinishedAtMs:          optInt64(s.FinishedAtMs),
	}
}

// InstanceFromDB maps a experiment_instance row.
func InstanceFromDB(i experimentdb.ExperimentInstance) *v1.ExperimentInstance {
	out := &v1.ExperimentInstance{
		Id:                       i.ID.String(),
		ExperimentId:             i.ExperimentID.String(),
		Label:                    i.Label,
		Playbook:                 i.Playbook,
		Symbol:                   i.Symbol,
		ConfigHash:               i.ConfigHash,
		PlaybookKey:              i.PlaybookKey,
		WorkspaceRelPath:         i.WorkspaceRelPath,
		Status:                   i.Status,
		Error:                    i.Error,
		StartedAtMs:              optInt64(i.StartedAtMs),
		FinishedAtMs:             optInt64(i.FinishedAtMs),
		CfgFastHr:                optInt32(i.CfgFastHr),
		CfgSlowHr:                optInt32(i.CfgSlowHr),
		CfgRsiBull:               optFloat64(i.CfgRsiBull),
		CfgRsiBear:               optFloat64(i.CfgRsiBear),
		CfgVolRatioBull:          optFloat64(i.CfgVolRatioBull),
		CfgVolRatioBear:          optFloat64(i.CfgVolRatioBear),
		CfgAtrStopMult:           optFloat64(i.CfgAtrStopMult),
		CfgMinBarsFlatAfterSell:  optInt32(i.CfgMinBarsFlatAfterSell),
		CfgMinBarsLongBeforeSell: optInt32(i.CfgMinBarsLongBeforeSell),
		Metrics: &v1.InstanceMetrics{
			DurationMs:        optInt64(i.MetricDurationMs),
			Cagr:              optFloat64(i.MetricCagr),
			MaxDrawdown:       optFloat64(i.MetricMaxDrawdown),
			Sharpe:            optFloat64(i.MetricSharpe),
			Sortino:           optFloat64(i.MetricSortino),
			MonthlyReturnAvg:  optFloat64(i.MetricMonthlyReturnAvg),
			MonthlyReturnMed:  optFloat64(i.MetricMonthlyReturnMed),
			YearlyReturnAvg:   optFloat64(i.MetricYearlyReturnAvg),
			YearlyReturnMed:   optFloat64(i.MetricYearlyReturnMed),
			TradesPerMonthAvg: optFloat64(i.MetricTradesPerMonthAvg),
			TradesPerMonthMed: optFloat64(i.MetricTradesPerMonthMed),
			WinRate:           optFloat32(i.MetricWinRate),
			MonthlyWinRateAvg: optFloat32(i.MetricMonthlyWinRateAvg),
			MonthlyWinRateMed: optFloat32(i.MetricMonthlyWinRateMed),
			YearlyWinRateAvg:  optFloat32(i.MetricYearlyWinRateAvg),
			YearlyWinRateMed:  optFloat32(i.MetricYearlyWinRateMed),
			Wins:              optInt32(i.MetricWins),
			Losses:            optInt32(i.MetricLosses),
			Breakeven:         optInt32(i.MetricBreakeven),
		},
	}
	switch i.Playbook {
	case "ma-cross":
		if i.CfgFastHr.Valid && i.CfgSlowHr.Valid {
			out.MaCross = &v1.MaCrossConfig{FastHr: i.CfgFastHr.Int32, SlowHr: i.CfgSlowHr.Int32}
		}
	case "signal-consensus":
		cfg := &v1.SignalConsensusConfig{}
		ok := false
		if i.CfgRsiBull.Valid {
			cfg.RsiBull = i.CfgRsiBull.Float64
			ok = true
		}
		if i.CfgRsiBear.Valid {
			cfg.RsiBear = i.CfgRsiBear.Float64
			ok = true
		}
		if i.CfgVolRatioBull.Valid {
			cfg.VolRatioBull = i.CfgVolRatioBull.Float64
			ok = true
		}
		if i.CfgVolRatioBear.Valid {
			cfg.VolRatioBear = i.CfgVolRatioBear.Float64
			ok = true
		}
		if i.CfgAtrStopMult.Valid {
			cfg.AtrStopMult = i.CfgAtrStopMult.Float64
			ok = true
		}
		if i.CfgMinBarsFlatAfterSell.Valid {
			v := i.CfgMinBarsFlatAfterSell.Int32
			cfg.MinBarsFlatAfterSell = &v
			ok = true
		}
		if i.CfgMinBarsLongBeforeSell.Valid {
			v := i.CfgMinBarsLongBeforeSell.Int32
			cfg.MinBarsLongBeforeSell = &v
			ok = true
		}
		if ok {
			out.SignalConsensus = cfg
		}
	}
	return out
}

// PeriodStatFromDB maps experiment_instance_period_stat.
func PeriodStatFromDB(p experimentdb.ExperimentInstancePeriodStat) *v1.ExperimentInstancePeriodStat {
	return &v1.ExperimentInstancePeriodStat{
		InstanceId: p.InstanceID.String(),
		PeriodKind: p.PeriodKind,
		PeriodKey:  p.PeriodKey,
		ReturnPct:  optFloat64(p.ReturnPct),
		TradeCount: optInt32(p.TradeCount),
	}
}
