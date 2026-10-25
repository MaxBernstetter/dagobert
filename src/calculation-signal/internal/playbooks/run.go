package playbooks

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	marketrunner "github.com/MaxBernstetter/dagobert/src/calculation-market-action/runner"
	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/retry"
	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/sizing"
	macross "github.com/MaxBernstetter/dagobert/src/calculation-signal/playbooks/ma-cross"
	signalconsensus "github.com/MaxBernstetter/dagobert/src/calculation-signal/playbooks/signal-consensus"
	"github.com/MaxBernstetter/dagobert/src/internal/instruments"
	"github.com/MaxBernstetter/dagobert/src/internal/kpi"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/paper"
)

// Run executes one strategy invocation for symbol at candleCloseMs (layer-1 signal only).
// strategyConfig is optional playbook JSON (incl. sizing); empty keeps the playbook defaults.
func Run(dataDir, playbookEnv, symbol string, candleCloseMs int64, strategyConfig json.RawMessage) (detail int, err error) {
	entry, err := Resolve(playbookEnv)
	if err != nil {
		return 40, err
	}
	strategyConfig, err = canonicalConfig(entry, strategyConfig)
	if err != nil {
		return 41, err
	}

	aggMS := framework.AggregationDurationMS()
	barOpen := candleCloseMs - aggMS
	if barOpen < 0 {
		return 10, errors.New("invalid candle close")
	}

	db, err := framework.OpenStore(dataDir)
	if err != nil {
		return 30, err
	}
	defer db.Close()

	if ok, err := framework.HasSignal(db, entry.ID, symbol, candleCloseMs); err != nil {
		return 30, err
	} else if ok {
		return 1, framework.ErrDuplicateSignal
	}

	var snap kpi.BarSnapshot
	err = retry.Do(func() error {
		snap, err = kpi.ReadSnapshot(dataDir, symbol, barOpen)
		return err
	})
	if errors.Is(err, kpi.ErrNotFound) {
		return 20, err
	}
	if err != nil {
		return 30, err
	}
	results := snap.Results

	prevBarOpen := barOpen - aggMS
	var prevResults []kpi.Result
	var prevBarClose float64
	hasPreviousBar := false
	if prevBarOpen >= 0 {
		var prevSnap kpi.BarSnapshot
		prevSnap, err = kpi.ReadSnapshot(dataDir, symbol, prevBarOpen)
		if err == nil {
			hasPreviousBar = true
			prevResults = prevSnap.Results
			prevBarClose = prevSnap.Close
		} else if !errors.Is(err, kpi.ErrNotFound) {
			return 30, err
		}
	}

	position, err := framework.DerivePosition(dataDir, db, entry.ID, symbol, candleCloseMs)
	if err != nil {
		return 30, err
	}

	var openUnits []sizing.Unit
	var lastFill, equity float64
	if len(strategyConfig) > 0 {
		openUnits, err = framework.DeriveOpenUnits(dataDir, db, entry.ID, symbol, candleCloseMs, marketrunner.PolicyFromEnv().MinOrderQuote)
		if err != nil {
			return 30, err
		}
		if len(openUnits) > 0 {
			position = "long"
			lastFill = openUnits[len(openUnits)-1].Fill
		}
		if equity, err = liveEquity(dataDir, symbol, snap.Close); err != nil {
			return 30, err
		}
	}

	gapFromPrevious := !hasPreviousBar && prevBarOpen > 0
	ctx := framework.Context{
		Symbol:           symbol,
		CandleClose:      candleCloseMs,
		BarOpen:          barOpen,
		BarClose:         snap.Close,
		BarVolume:        snap.Volume,
		PreviousBarClose: prevBarClose,
		BarsSinceBuy:     999,
		BarsSinceSell:    999,
		Position:         position,
		Results:          results,
		PreviousResults:  prevResults,
		HasPreviousBar:   hasPreviousBar,
		PreviousBarOpen:  prevBarOpen,
		GapFromPrevious:  gapFromPrevious,
		Equity:           equity,
		OpenUnits:        openUnits,
		LastFill:         lastFill,
		StrategyConfig:   strategyConfig,
	}

	dec, err := entry.Strategy.Evaluate(ctx)
	if err != nil {
		return 30, err
	}

	signalJSON, err := encodeSignalWithConfig(entry, symbol, dec, strategyConfig)
	if err != nil {
		return 30, err
	}

	if err := framework.CommitSignal(db, entry.ID, symbol, candleCloseMs, signalJSON); err != nil {
		if errors.Is(err, framework.ErrDuplicateSignal) {
			return 1, err
		}
		return 30, err
	}
	return 0, nil
}

func encodeSignal(entry framework.Entry, symbol string, dec framework.Decision) ([]byte, error) {
	return encodeSignalWithConfig(entry, symbol, dec, nil)
}

func encodeSignalWithConfig(entry framework.Entry, symbol string, dec framework.Decision, configJSON []byte) ([]byte, error) {
	inst, _ := instruments.Resolve(symbol)
	hashInput := entry.Strategy.Slug() + ":" + entry.Strategy.LogicVersion()
	if len(bytes.TrimSpace(configJSON)) > 0 {
		hashInput += ":" + string(configJSON)
	}
	hash := sha256.Sum256([]byte(hashInput))
	body := map[string]any{
		"decision":      dec.Decision,
		"exit":          dec.Exit,
		"rule":          dec.RuleID,
		"written_at":    time.Now().UTC().Format(time.RFC3339),
		"venue":         inst.Venue,
		"venue_symbol":  inst.VenueSymbol,
		"price_source":  inst.PriceSource,
		"playbook_hash": hex.EncodeToString(hash[:]),
		"algo":          "v1",
	}
	// Risk-sized amounts are omitted when unset so legacy signals keep their shape.
	for k, v := range map[string]*float64{
		"quote_notional": dec.QuoteNotional, "base_qty": dec.BaseQty, "stop_price": dec.StopPrice,
		"raise_stops_by": dec.RaiseStopsBy, "requested_risk_pct": dec.RequestedRiskPct, "capped_risk_pct": dec.CappedRiskPct,
	} {
		if v != nil {
			body[k] = *v
		}
	}
	return json.Marshal(body)
}

// canonicalConfig validates strategy config for the playbook and returns its canonical JSON
// (nil when raw is empty, so unconfigured runs stay unchanged).
func canonicalConfig(entry framework.Entry, raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	aggMS := framework.AggregationDurationMS()
	switch entry.Strategy.Slug() {
	case "ma-cross":
		cfg, err := macross.ParseConfig(raw)
		if err != nil {
			return nil, err
		}
		if err := cfg.Validate(aggMS); err != nil {
			return nil, err
		}
		return cfg.CanonicalJSON()
	case "signal-consensus":
		cfg, err := signalconsensus.ParseConfig(raw)
		if err != nil {
			return nil, err
		}
		if err := cfg.Validate(aggMS); err != nil {
			return nil, err
		}
		return cfg.CanonicalJSON()
	}
	return nil, errors.New("strategy config not supported by this playbook")
}

// liveEquity is quote cash plus base marked at the bar close. EQUITY (quote units) overrides
// the paper wallet; with neither it is 0 and a sized entry is skipped.
func liveEquity(dataDir, symbol string, mark float64) (float64, error) {
	if v := strings.TrimSpace(os.Getenv("EQUITY")); v != "" {
		return strconv.ParseFloat(v, 64)
	}
	inst, ok := instruments.Resolve(symbol)
	if !ok || !paper.Enabled() || mark <= 0 {
		return 0, nil
	}
	q, b, err := paper.QuoteAndBase(dataDir, inst.Quote, inst.Base)
	if err != nil {
		return 0, err
	}
	return q + b*mark, nil
}
