package action

import (
	"database/sql"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/MaxBernstetter/trading-copilot/instruments"
	"github.com/MaxBernstetter/trading-copilot/services/strategy/internal/playbook"
)

var ErrDuplicate = errors.New("action already stored")

const createTableSQL = `
CREATE TABLE IF NOT EXISTS action (
  playbook TEXT NOT NULL,
  symbol TEXT NOT NULL,
  candle_close INTEGER NOT NULL,
  body TEXT NOT NULL,
  PRIMARY KEY (playbook, symbol, candle_close)
);
`

type bodyRecord struct {
	Decision     string         `json:"decision"`
	Size         *float64       `json:"size"`
	Unit         string         `json:"unit,omitempty"`
	UnitKind     string         `json:"unit_kind,omitempty"`
	Exit         *playbook.Exit `json:"exit"`
	Rule         string         `json:"rule"`
	WrittenAt    string         `json:"written_at,omitempty"`
	Venue        string         `json:"venue,omitempty"`
	VenueSymbol  string         `json:"venue_symbol,omitempty"`
	PriceSource  string         `json:"price_source,omitempty"`
	PlaybookHash string         `json:"playbook_hash,omitempty"`
	Algo         string         `json:"algo,omitempty"`
}

// Write inserts one action row. Playbook identity is the base filename of the playbook path.
func Write(dataDir, playbookPath, symbol string, candleClose int64, dec playbook.Decision) error {
	// Idempotency key uses the playbook file basename (e.g. ma-cross.yaml), not a content hash.
	playbookID := filepath.Base(playbookPath)

	raw, err := os.ReadFile(playbookPath)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	inst, _ := instruments.BySeries(symbol)
	unit, unitKind := "", ""
	switch dec.Decision {
	case "buy":
		unit, unitKind = inst.Quote, "quote"
	case "sell":
		unit, unitKind = inst.Base, "base"
	}
	body := bodyRecord{
		Decision:     dec.Decision,
		Size:         dec.Size,
		Unit:         unit,
		UnitKind:     unitKind,
		Exit:         dec.Exit,
		Rule:         dec.RuleID,
		WrittenAt:    time.Now().UTC().Format(time.RFC3339),
		Venue:        inst.Venue,
		VenueSymbol:  inst.VenueSymbol,
		PriceSource:  inst.PriceSource,
		PlaybookHash: hex.EncodeToString(sum[:]),
		Algo:         "v1",
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	path := filepath.Join(dataDir, "strategy.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(createTableSQL); err != nil {
		return err
	}

	_, err = db.Exec(
		`INSERT INTO action (playbook, symbol, candle_close, body) VALUES (?, ?, ?, ?)`,
		playbookID, symbol, candleClose, string(bodyJSON),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func isUniqueViolation(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "constraint failed")
}
