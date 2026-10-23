package framework

import (
	"database/sql"
	"encoding/json"

	"github.com/MaxBernstetter/trading-copilot/src/internal/venue/paper"
)

// DerivePosition rebuilds long/flat before the current candle.
// In simulate mode, position follows execution fills so failed orders do not leave a phantom long.
func DerivePosition(dataDir string, db *sql.DB, playbookID, symbol string, beforeCandleClose int64) (string, error) {
	if paper.Enabled() {
		return derivePositionFromFills(dataDir, playbookID, symbol, beforeCandleClose)
	}
	return derivePositionFromSignals(db, playbookID, symbol, beforeCandleClose)
}

func derivePositionFromSignals(db *sql.DB, playbookID, symbol string, beforeCandleClose int64) (string, error) {
	rows, err := db.Query(
		`SELECT body FROM signal
		 WHERE playbook = ? AND symbol = ? AND candle_close < ?
		 ORDER BY candle_close ASC`,
		playbookID, symbol, beforeCandleClose,
	)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	position := "flat"
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return "", err
		}
		var body struct {
			Decision string `json:"decision"`
		}
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			return "", err
		}
		switch body.Decision {
		case "buy":
			position = "long"
		case "sell":
			position = "flat"
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return position, nil
}
