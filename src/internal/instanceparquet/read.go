package instanceparquet

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/parquet-go/parquet-go"
)

// The Read functions take a shard directory and an instance id and read only that instance's row
// group. They return empty rows and an error satisfying errors.Is(err, fs.ErrNotExist) when the file
// is missing or the instance has no rows in it (no row group). Callers decide whether that is a
// valid empty result. Each reader decodes only the columns of its row type.

// ReadFills reads the fills of one instance.
func ReadFills(shardDir, instanceID string) ([]Fill, error) {
	return readInstance[Fill](shardDir, FillsFile, instanceID)
}

// ReadOrders reads the orders of one instance.
func ReadOrders(shardDir, instanceID string) ([]Order, error) {
	return readInstance[Order](shardDir, OrdersFile, instanceID)
}

// ReadJournals reads the fill journals of one instance.
func ReadJournals(shardDir, instanceID string) ([]Journal, error) {
	return readInstance[Journal](shardDir, JournalsFile, instanceID)
}

// ReadTradeOutcomes reads the trade outcomes of one instance.
func ReadTradeOutcomes(shardDir, instanceID string) ([]TradeOutcome, error) {
	return readInstance[TradeOutcome](shardDir, TradeOutcomesFile, instanceID)
}

// ReadPeriodStats reads the period stats of one instance.
func ReadPeriodStats(shardDir, instanceID string) ([]PeriodStat, error) {
	return readInstance[PeriodStat](shardDir, PeriodStatsFile, instanceID)
}

// ReadReport reads the report JSON of one instance.
func ReadReport(shardDir, instanceID string) ([]byte, error) {
	rows, err := readInstance[Report](shardDir, ReportsFile, instanceID)
	if err != nil {
		return nil, err
	}
	return []byte(rows[0].ReportJSON), nil
}

func readInstance[T any](shardDir, name, instanceID string) (rows []T, err error) {
	rows = []T{}
	f, err := os.Open(filepath.Join(shardDir, name))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return rows, err
		}
		return rows, fmt.Errorf("read %s: %w", name, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return rows, fmt.Errorf("read %s: %w", name, err)
	}
	pf, err := parquet.OpenFile(f, info.Size())
	if err != nil {
		return rows, fmt.Errorf("read %s: %w", name, err)
	}
	raw, _ := pf.Lookup(KeyInstances)
	var index map[string]int
	if err := json.Unmarshal([]byte(raw), &index); err != nil {
		return rows, fmt.Errorf("read %s: footer %s: %w", name, KeyInstances, err)
	}
	g, ok := index[instanceID]
	groups := pf.RowGroups()
	if !ok {
		return rows, fmt.Errorf("read %s: instance %s: %w", name, instanceID, fs.ErrNotExist)
	}
	if g < 0 || g >= len(groups) {
		return rows, fmt.Errorf("read %s: instance %s: row group %d out of range", name, instanceID, g)
	}
	rg := groups[g]
	rows = make([]T, rg.NumRows())
	r := parquet.NewGenericRowGroupReader[T](rg)
	defer r.Close()
	for n := 0; n < len(rows); {
		m, err := r.Read(rows[n:])
		n += m
		if err != nil && !errors.Is(err, io.EOF) {
			return []T{}, fmt.Errorf("read %s: %w", name, err)
		}
		if err != nil && n < len(rows) {
			return []T{}, fmt.Errorf("read %s: short row group", name)
		}
	}
	return rows, nil
}
