package instanceparquet

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/parquet-go/parquet-go"
)

// The Write functions replace a file atomically: rows go to a temporary file in the same
// directory, which is fsynced and renamed over the target. A failed write leaves the previous
// file, if any, untouched. Each file holds exactly one row group.

// WriteFills writes fills.parquet. Rows with an empty side are skipped; sides are stored lowercase.
func WriteFills(dir string, meta Meta, rows []Fill) error {
	out := rows[:0:0]
	for _, r := range rows {
		if r.Side == "" {
			continue
		}
		r.Side = strings.ToLower(r.Side)
		out = append(out, r)
	}
	return writeFile(dir, FillsFile, meta, out)
}

// WriteOrders writes orders.parquet.
func WriteOrders(dir string, meta Meta, rows []Order) error {
	return writeFile(dir, OrdersFile, meta, rows)
}

// WriteJournals writes journals.parquet.
func WriteJournals(dir string, meta Meta, rows []Journal) error {
	return writeFile(dir, JournalsFile, meta, rows)
}

// WriteTradeOutcomes writes trade_outcomes.parquet.
func WriteTradeOutcomes(dir string, meta Meta, rows []TradeOutcome) error {
	return writeFile(dir, TradeOutcomesFile, meta, rows)
}

// WritePeriodStats writes period_stats.parquet.
func WritePeriodStats(dir string, meta Meta, rows []PeriodStat) error {
	return writeFile(dir, PeriodStatsFile, meta, rows)
}

func writeFile[T any](dir, name string, meta Meta, rows []T) (err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+name+".*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	// One row group per file: the group size is the row count, never zero.
	opts := append(footer(meta), parquet.MaxRowsPerRowGroup(int64(max(len(rows), 1))))
	w := parquet.NewGenericWriter[T](tmp, opts...)
	if _, err = w.Write(rows); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("close %s: %w", name, err)
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, name))
}
