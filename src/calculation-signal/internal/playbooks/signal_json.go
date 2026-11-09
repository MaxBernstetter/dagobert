package playbooks

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
	"github.com/MaxBernstetter/dagobert/src/internal/instruments"
)

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
