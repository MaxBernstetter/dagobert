package pgcatalog

import (
	"encoding/json"

	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	"github.com/jackc/pgx/v5/pgtype"
)

func experimentRecordFromRow(s experimentdb.Experiment, insts []experimentdb.ExperimentInstance) store.ExperimentRecord {
	rec := store.ExperimentRecord{
		ID:                    s.ID.String(),
		Name:                  s.Name,
		Description:           s.Description,
		Status:                s.Status,
		Error:                 s.Error,
		CreatedAt:             s.CreatedAtMs,
		ProgressStage:         int(s.ProgressStage),
		ProgressStages:        int(s.ProgressStages),
		ProgressStageLabel:    s.ProgressStageLabel,
		ProgressInstanceDone:  int(s.ProgressInstanceDone),
		ProgressInstanceTotal: int(s.ProgressInstanceTotal),
		Settings: store.ExperimentSettings{
			Provider:          s.Provider,
			Base:              s.Base,
			Quote:             s.Quote,
			FromMs:            s.FromMs,
			ToMs:              s.ToMs,
			InitialUSDC:       s.InitialUsdc,
			InitialUSDT:       s.InitialUsdt,
			InitialBase:       s.InitialBase,
			TargetWeightPct:   s.TargetWeightPct,
			CashReserveQuote:  s.CashReserveQuote,
			MinOrderQuote:     s.MinOrderQuote,
			RedownloadMissing: s.RedownloadMissing,
			Algo:              s.Algo,
			SpreadBps:         s.SpreadBps,
			SlippageBps:       s.SlippageBps,
			FeeBps:            s.FeeBps,
		},
	}
	if s.StartedAtMs.Valid {
		v := s.StartedAtMs.Int64
		rec.StartedAt = &v
	}
	if s.FinishedAtMs.Valid {
		v := s.FinishedAtMs.Int64
		rec.FinishedAt = &v
	}
	out := make([]store.ExperimentInstance, 0, len(insts))
	for _, i := range insts {
		out = append(out, instanceFromRow(i))
	}
	rec.Instances = out
	return rec
}

func instanceFromRow(i experimentdb.ExperimentInstance) store.ExperimentInstance {
	inst := store.ExperimentInstance{
		ID:               i.ID.String(),
		ExperimentID:     i.ExperimentID.String(),
		Label:            i.Label,
		Playbook:         i.Playbook,
		Symbol:           i.Symbol,
		ConfigHash:       i.ConfigHash,
		PlaybookKey:      i.PlaybookKey,
		WorkspaceRelPath: i.WorkspaceRelPath,
		Status:           i.Status,
		Error:            i.Error,
		StrategyConfig:   strategyConfigJSON(i),
	}
	if i.StartedAtMs.Valid {
		v := i.StartedAtMs.Int64
		inst.StartedAt = &v
	}
	if i.FinishedAtMs.Valid {
		v := i.FinishedAtMs.Int64
		inst.FinishedAt = &v
	}
	return inst
}

func strategyConfigJSON(i experimentdb.ExperimentInstance) json.RawMessage {
	if i.StrategyConfigJson.Valid {
		return json.RawMessage(i.StrategyConfigJson.String)
	}
	switch i.Playbook {
	case "ma-cross":
		if i.CfgFastHr.Valid && i.CfgSlowHr.Valid {
			b, _ := json.Marshal(map[string]int32{
				"fastHr": i.CfgFastHr.Int32,
				"slowHr": i.CfgSlowHr.Int32,
			})
			return b
		}
	case "signal-consensus":
		m := map[string]interface{}{}
		if i.CfgRsiBull.Valid {
			m["rsiBull"] = i.CfgRsiBull.Float64
		}
		if i.CfgRsiBear.Valid {
			m["rsiBear"] = i.CfgRsiBear.Float64
		}
		if i.CfgVolRatioBull.Valid {
			m["volRatioBull"] = i.CfgVolRatioBull.Float64
		}
		if i.CfgVolRatioBear.Valid {
			m["volRatioBear"] = i.CfgVolRatioBear.Float64
		}
		if i.CfgAtrStopMult.Valid {
			m["atrStopMult"] = i.CfgAtrStopMult.Float64
		}
		if i.CfgMinBarsFlatAfterSell.Valid {
			m["minBarsFlatAfterSell"] = i.CfgMinBarsFlatAfterSell.Int32
		}
		if i.CfgMinBarsLongBeforeSell.Valid {
			m["minBarsLongBeforeSell"] = i.CfgMinBarsLongBeforeSell.Int32
		}
		if len(m) > 0 {
			b, _ := json.Marshal(m)
			return b
		}
	}
	return json.RawMessage("{}")
}

func timingInt8(ms int64) pgtype.Int8 {
	if ms == 0 {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: ms, Valid: true}
}

func float8Ptr(p *float64) pgtype.Float8 {
	if p == nil {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: *p, Valid: true}
}

func float4Ptr(p *float64) pgtype.Float4 {
	if p == nil {
		return pgtype.Float4{}
	}
	return pgtype.Float4{Float32: float32(*p), Valid: true}
}

func int4Ptr(p *int32) pgtype.Int4 {
	if p == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *p, Valid: true}
}

func int8Ptr(p *int64) pgtype.Int8 {
	if p == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *p, Valid: true}
}
