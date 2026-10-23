package allocate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/MaxBernstetter/trading-copilot/src/internal/instruments"
)

// EncodeActionJSON builds the action row body (same shape as the market-action job).
func EncodeActionJSON(playbookID, symbol, decision string, size *float64, rule, signalRule string) ([]byte, error) {
	inst, _ := instruments.BySeries(symbol)
	unit, unitKind := "", ""
	switch decision {
	case "buy":
		unit, unitKind = inst.Quote, "quote"
	case "sell":
		unit, unitKind = inst.Base, "base"
	}
	hash := sha256.Sum256([]byte("ma-cross:3"))
	body := map[string]any{
		"decision":      decision,
		"size":          size,
		"unit":          unit,
		"unit_kind":     unitKind,
		"rule":          rule,
		"signal_rule":   signalRule,
		"written_at":    time.Now().UTC().Format(time.RFC3339),
		"venue":         inst.Venue,
		"venue_symbol":  inst.VenueSymbol,
		"price_source":  inst.PriceSource,
		"playbook_hash": hex.EncodeToString(hash[:]),
		"algo":          "v1",
	}
	return json.Marshal(body)
}
