package instanceparquet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"

	"github.com/parquet-go/parquet-go"

	"github.com/MaxBernstetter/dagobert/src/internal/workspace"
)

// The Read functions take the data dir, the shard key (the shard directory relative to it) and an
// instance id and read only that instance's row group. The shard file is opened through the
// workspace store: a local file with LAKE_BACKEND=fs, a downloaded object with s3. They return empty rows and an error satisfying errors.Is(err, fs.ErrNotExist) when the file
// is missing or the instance has no rows in it (no row group). Callers decide whether that is a
// valid empty result. Each reader decodes only the columns of its row type.

// ReadFills reads the fills of one instance.
func ReadFills(ctx context.Context, dataDir, shardKey, instanceID string) ([]Fill, error) {
	return readInstance[Fill](ctx, dataDir, shardKey, FillsFile, instanceID)
}

// ReadOrders reads the orders of one instance.
func ReadOrders(ctx context.Context, dataDir, shardKey, instanceID string) ([]Order, error) {
	return readInstance[Order](ctx, dataDir, shardKey, OrdersFile, instanceID)
}

// ReadJournals reads the fill journals of one instance.
func ReadJournals(ctx context.Context, dataDir, shardKey, instanceID string) ([]Journal, error) {
	return readInstance[Journal](ctx, dataDir, shardKey, JournalsFile, instanceID)
}

// ReadTradeOutcomes reads the trade outcomes of one instance.
func ReadTradeOutcomes(ctx context.Context, dataDir, shardKey, instanceID string) ([]TradeOutcome, error) {
	return readInstance[TradeOutcome](ctx, dataDir, shardKey, TradeOutcomesFile, instanceID)
}

// ReadPeriodStats reads the period stats of one instance.
func ReadPeriodStats(ctx context.Context, dataDir, shardKey, instanceID string) ([]PeriodStat, error) {
	return readInstance[PeriodStat](ctx, dataDir, shardKey, PeriodStatsFile, instanceID)
}

// ReadReport reads the report JSON of one instance.
func ReadReport(ctx context.Context, dataDir, shardKey, instanceID string) ([]byte, error) {
	rows, err := readInstance[Report](ctx, dataDir, shardKey, ReportsFile, instanceID)
	if err != nil {
		return nil, err
	}
	return []byte(rows[0].ReportJSON), nil
}

func readInstance[T any](ctx context.Context, dataDir, shardKey, name, instanceID string) (rows []T, err error) {
	rows = []T{}
	f, size, err := workspace.Open(ctx, dataDir, path.Join(shardKey, name))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return rows, err
		}
		return rows, fmt.Errorf("read %s: %w", name, err)
	}
	defer f.Close()
	pf, err := parquet.OpenFile(f, size)
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
