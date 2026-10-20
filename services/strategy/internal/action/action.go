package action

import (
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

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
	Decision  string         `json:"decision"`
	Size      *float64       `json:"size"`
	Exit      *playbook.Exit `json:"exit"`
	Rule      string         `json:"rule"`
	WrittenAt string         `json:"written_at,omitempty"`
}

// Write inserts one action row. Playbook identity is the base filename of the playbook path.
func Write(dataDir, playbookPath, symbol string, candleClose int64, dec playbook.Decision) error {
	// Idempotency key uses the playbook file basename (e.g. ma-cross.yaml), not a content hash.
	playbookID := filepath.Base(playbookPath)

	body := bodyRecord{
		Decision:  dec.Decision,
		Size:      dec.Size,
		Exit:      dec.Exit,
		Rule:      dec.RuleID,
		WrittenAt: time.Now().UTC().Format(time.RFC3339),
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
