package strategyaction

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/MaxBernstetter/dagobert/src/internal/signalstore"
)

var ErrNotFound = errors.New("action not found")

type Action struct {
	Playbook    string
	Symbol      string
	CandleClose int64
	Decision    string
	Size        *float64
}

type bodyRecord struct {
	Decision string   `json:"decision"`
	Size     *float64 `json:"size"`
}

func Read(dataDir, playbookPath, symbol string, candleClose int64) (Action, error) {
	playbookID := filepath.Base(playbookPath)
	path := signalstore.SQLitePath(dataDir)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return Action{}, ErrNotFound
		}
		return Action{}, err
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return Action{}, err
	}
	defer db.Close()

	var bodyJSON string
	err = db.QueryRow(
		`SELECT body FROM action WHERE playbook = ? AND symbol = ? AND candle_close = ?`,
		playbookID, symbol, candleClose,
	).Scan(&bodyJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return Action{}, ErrNotFound
	}
	if err != nil {
		return Action{}, err
	}

	var body bodyRecord
	if err := json.Unmarshal([]byte(bodyJSON), &body); err != nil {
		return Action{}, err
	}

	return Action{
		Playbook:    playbookID,
		Symbol:      symbol,
		CandleClose: candleClose,
		Decision:    body.Decision,
		Size:        body.Size,
	}, nil
}
