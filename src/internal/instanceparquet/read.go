package instanceparquet

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/parquet-go/parquet-go"
)

// The Read functions return empty rows and an error satisfying errors.Is(err, fs.ErrNotExist)
// when the file is missing. Callers decide whether that is a valid empty result.
// Each reader decodes only the columns of its row type; a file written with more columns is read
// with the subset, as parquet-go matches columns by name.

// ReadFills reads fills.parquet.
func ReadFills(dir string) ([]Fill, error) { return readFile[Fill](dir, FillsFile) }

// ReadOrders reads orders.parquet.
func ReadOrders(dir string) ([]Order, error) { return readFile[Order](dir, OrdersFile) }

// ReadJournals reads journals.parquet.
func ReadJournals(dir string) ([]Journal, error) { return readFile[Journal](dir, JournalsFile) }

// ReadTradeOutcomes reads trade_outcomes.parquet.
func ReadTradeOutcomes(dir string) ([]TradeOutcome, error) {
	return readFile[TradeOutcome](dir, TradeOutcomesFile)
}

// ReadPeriodStats reads period_stats.parquet.
func ReadPeriodStats(dir string) ([]PeriodStat, error) {
	return readFile[PeriodStat](dir, PeriodStatsFile)
}

func readFile[T any](dir, name string) ([]T, error) {
	path := filepath.Join(dir, name)
	rows, err := parquet.ReadFile[T](path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []T{}, err
		}
		return []T{}, fmt.Errorf("read %s: %w", name, err)
	}
	if rows == nil {
		rows = []T{}
	}
	return rows, nil
}
