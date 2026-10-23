package framework

import (
	"database/sql"

	"github.com/MaxBernstetter/trading-copilot/src/internal/signalstore"
)

var (
	ErrDuplicateAction = signalstore.ErrDuplicateAction
	ErrDuplicateSignal = signalstore.ErrDuplicateSignal
	ErrNotFound        = signalstore.ErrNotFound
)

type Signal = signalstore.Signal

func OpenStore(dataDir string) (*sql.DB, error) {
	return signalstore.Open(dataDir)
}

func HasAction(db *sql.DB, playbookID, symbol string, candleClose int64) (bool, error) {
	return signalstore.HasAction(db, playbookID, symbol, candleClose)
}

func HasSignal(db *sql.DB, playbookID, symbol string, candleClose int64) (bool, error) {
	return signalstore.HasSignal(db, playbookID, symbol, candleClose)
}

func ReadSignal(db *sql.DB, playbookID, symbol string, candleClose int64) (Signal, error) {
	return signalstore.ReadSignal(db, playbookID, symbol, candleClose)
}

func CommitSignal(db *sql.DB, playbookID, symbol string, candleClose int64, body []byte) error {
	return signalstore.CommitSignal(db, playbookID, symbol, candleClose, body)
}

func CommitAction(db *sql.DB, playbookID, symbol string, candleClose int64, actionBody []byte) error {
	return signalstore.CommitAction(db, playbookID, symbol, candleClose, actionBody)
}

func AggregationDurationMS() int64 {
	return signalstore.AggregationDurationMS()
}
