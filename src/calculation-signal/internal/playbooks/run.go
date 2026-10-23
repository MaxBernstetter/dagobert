package playbooks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/MaxBernstetter/trading-copilot/src/internal/instruments"
	"github.com/MaxBernstetter/trading-copilot/src/calculation-signal/internal/framework"
	"github.com/MaxBernstetter/trading-copilot/src/internal/kpi"
	"github.com/MaxBernstetter/trading-copilot/src/calculation-signal/internal/retry"
)

// Run executes one strategy invocation for symbol at candleCloseMs (layer-1 signal only).
func Run(dataDir, playbookEnv, symbol string, candleCloseMs int64) (detail int, err error) {
	entry, err := Resolve(playbookEnv)
	if err != nil {
		return 40, err
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
	}

	dec, err := entry.Strategy.Evaluate(ctx)
	if err != nil {
		return 30, err
	}

	signalJSON, err := encodeSignal(entry, symbol, dec)
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
	inst, _ := instruments.BySeries(symbol)
	hash := sha256.Sum256([]byte(entry.Strategy.Slug() + ":" + entry.Strategy.LogicVersion()))
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
	return json.Marshal(body)
}
