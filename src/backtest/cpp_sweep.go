package backtest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"

	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	macross "github.com/MaxBernstetter/dagobert/src/calculation-signal/playbooks/ma-cross"
	signalconsensus "github.com/MaxBernstetter/dagobert/src/calculation-signal/playbooks/signal-consensus"
	signalrunner "github.com/MaxBernstetter/dagobert/src/calculation-signal/runner"
	tirunner "github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/runner"
	pb "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
)

// C++ sweep client (issue #130, S4a). It turns experiment instances into one SweepRequest
// (shared bars, one item per instance), runs a strategy executable in --sweep mode and converts
// the reported actions into signalrunner.ReplayAction for the KPI worker. The executable only
// computes actions; fills, equity and KPIs are rebuilt in Go (ReplayKPI).

const sweepSchemaVersion = 2

// sweepLabel is the RunEnvelope label of a playbook slug the C++ path supports.
func sweepLabel(slug string) (string, error) {
	switch slug {
	case "ma-cross":
		return "sma-cross", nil
	case "signal-consensus":
		return "signal-consensus", nil
	}
	return "", fmt.Errorf("C++ engine does not support playbook %q", slug)
}

// SweepPayload maps an instance's StrategyConfig to the C++ config message (Any).
// Only ma-cross and signal-consensus are supported; any other playbook is an error, never a
// silent fallback. The full resolved config (defaults applied) is sent.
func SweepPayload(slug string, strategyConfig []byte) (*anypb.Any, error) {
	if _, err := sweepLabel(slug); err != nil {
		return nil, err
	}
	if slug == "signal-consensus" {
		cfg, err := signalconsensus.ParseConfig(strategyConfig)
		if err != nil {
			return nil, err
		}
		if err := cfg.Validate(0); err != nil {
			return nil, err
		}
		// proto3 cannot tell an explicit 0 from unset and C++ reads 0 as the default (8/4);
		// Go honours 0. Reject until explicit presence exists (#128).
		if cfg.MinBarsFlatAfterSell == 0 || cfg.MinBarsLongBeforeSell == 0 {
			return nil, fmt.Errorf("C++ engine: signal-consensus cooldown 0 is not supported yet (#128); use >= 1 or EXPERIMENT_ENGINE=go")
		}
		msg := &pb.SignalConsensusConfig{
			RsiBull: cfg.RsiBull, RsiBear: cfg.RsiBear,
			VolRatioBull: cfg.VolRatioBull, VolRatioBear: cfg.VolRatioBear,
			AtrStopMult:           cfg.AtrStopMult,
			MinBarsFlatAfterSell:  int32(cfg.MinBarsFlatAfterSell),
			MinBarsLongBeforeSell: int32(cfg.MinBarsLongBeforeSell),
		}
		if cfg.Sizing != nil {
			msg.Sizing = cfg.Sizing.Proto()
		}
		return anypb.New(msg)
	}
	cfg, err := macross.ParseConfig(strategyConfig)
	if err != nil {
		return nil, err
	}
	msg := &pb.MaCrossConfig{FastHr: int32(cfg.FastHr), SlowHr: int32(cfg.SlowHr)}
	if cfg.Sizing != nil {
		msg.Sizing = cfg.Sizing.Proto()
	}
	return anypb.New(msg)
}

// SweepItemSpec is one instance of a sweep. Key is echoed in the report.
type SweepItemSpec struct {
	Key            string
	Playbook       string // slug, e.g. "ma-cross"
	StrategyConfig []byte
}

// BuildSweepRequest builds the request for items that share one symbol and bar range.
// All items must be of the same playbook (one executable per playbook).
func BuildSweepRequest(settings store.ExperimentSettings, base, quote string, r tirunner.BarRange, initialQuote float64, items []SweepItemSpec) (*pb.SweepRequest, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("sweep has no items")
	}
	if len(r.Bars) == 0 {
		return nil, fmt.Errorf("sweep has no bars")
	}
	label, err := sweepLabel(items[0].Playbook)
	if err != nil {
		return nil, err
	}
	req := &pb.SweepRequest{
		Base: &pb.RunEnvelope{
			SchemaVersion: sweepSchemaVersion, Label: label, Provider: settings.Provider,
			Base: base, Quote: quote, InitialQuote: initialQuote,
		},
		TradeFromMs: r.TradeFromMs,
	}
	for _, b := range r.Bars {
		req.Bars = append(req.Bars, &pb.Bar{
			OpenMs: b.OpenMS, Open: b.Open, High: b.High, Low: b.Low, Close: b.Close,
			Volume: b.Volume, DurationMs: b.DurationMS, SegmentClose: b.SegmentClose,
		})
	}
	for _, it := range items {
		if it.Playbook != items[0].Playbook {
			return nil, fmt.Errorf("sweep mixes playbooks %q and %q", items[0].Playbook, it.Playbook)
		}
		payload, err := SweepPayload(it.Playbook, it.StrategyConfig)
		if err != nil {
			return nil, fmt.Errorf("item %s: %w", it.Key, err)
		}
		req.Items = append(req.Items, &pb.SweepItem{Key: it.Key, Payload: payload})
	}
	return req, nil
}

// RunSweep runs `bin --sweep` with req on stdin and returns one report per item, in order.
// A non-zero exit, a short or unreadable reply fails the whole call (a crashed shard fails its
// instances); a per-item error is left in RunReport.Error for the caller to apply per instance.
func RunSweep(ctx context.Context, bin string, req *pb.SweepRequest) ([]*pb.RunReport, error) {
	in, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, bin, "--sweep")
	cmd.Stdin = bytes.NewReader(in)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s --sweep: %w: %s", bin, err, bytes.TrimSpace(stderr.Bytes()))
	}
	reports := make([]*pb.RunReport, 0, len(req.Items))
	for {
		rep := &pb.RunReport{}
		err := protodelim.UnmarshalFrom(&stdout, rep)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s --sweep: bad reply: %w", bin, err)
		}
		reports = append(reports, rep)
	}
	if len(reports) != len(req.Items) {
		return nil, fmt.Errorf("%s --sweep: %d reports for %d items", bin, len(reports), len(req.Items))
	}
	for i, rep := range reports {
		if rep.Key != req.Items[i].Key {
			return nil, fmt.Errorf("%s --sweep: report %d has key %q, want %q", bin, i, rep.Key, req.Items[i].Key)
		}
	}
	return reports, nil
}

// ReplayActions converts a report's actions into ReplayActions. bar_open_ms is turned into the
// candle close with aggMS. Sized actions (quote_notional or base_qty) must carry a positive
// equity; unsized actions keep the legacy Go sizing (nil quantities).
func ReplayActions(rep *pb.RunReport, aggMS int64) ([]signalrunner.ReplayAction, error) {
	if rep.Error != "" {
		return nil, fmt.Errorf("sweep item %s: %s", rep.Key, rep.Error)
	}
	out := make([]signalrunner.ReplayAction, 0, len(rep.Actions))
	for _, a := range rep.Actions {
		if a.Side != "buy" && a.Side != "sell" {
			continue
		}
		sz := &signalrunner.ReplaySizing{QuoteNotional: a.QuoteNotional, BaseQty: a.BaseQty}
		if a.QuoteNotional != nil || a.BaseQty != nil {
			if a.Equity == nil || *a.Equity <= 0 {
				return nil, fmt.Errorf("sweep item %s: sized %s action at bar %d has no positive equity", rep.Key, a.Side, a.BarOpenMs)
			}
			sz.Equity = *a.Equity
			if a.BaseQty != nil {
				sz.HeatBasePerEquity = a.HeatBasePerEquity
			}
		}
		out = append(out, signalrunner.ReplayAction{
			CandleCloseMs: a.BarOpenMs + aggMS, Decision: a.Side, Sizing: sz,
		})
	}
	return out, nil
}
