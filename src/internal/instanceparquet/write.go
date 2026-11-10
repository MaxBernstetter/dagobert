package instanceparquet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/parquet-go/parquet-go"
)

// WriteShard writes the six files of one shard into dir, one row group per instance and file. Each
// file is replaced atomically: rows go to a temporary file in dir, which is fsynced and renamed over
// the target. The set of files is not atomic; on error the caller removes the whole shard directory.
// Fills with an empty side are skipped; sides are stored lowercase.
func WriteShard(dir, experimentID string, data []InstanceData) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	ids := make([]string, len(data))
	for i, d := range data {
		ids[i] = d.InstanceID
	}
	if err := writeFile(dir, ReportsFile, experimentID, ids, func(i int) []Report {
		return []Report{{InstanceID: data[i].InstanceID, ReportJSON: string(data[i].ReportJSON)}}
	}); err != nil {
		return err
	}
	if err := writeFile(dir, FillsFile, experimentID, ids, func(i int) []Fill { return normalizedFills(data[i].Fills) }); err != nil {
		return err
	}
	if err := writeFile(dir, OrdersFile, experimentID, ids, func(i int) []Order { return data[i].Orders }); err != nil {
		return err
	}
	if err := writeFile(dir, JournalsFile, experimentID, ids, func(i int) []Journal { return data[i].Journals }); err != nil {
		return err
	}
	if err := writeFile(dir, TradeOutcomesFile, experimentID, ids, func(i int) []TradeOutcome { return data[i].Outcomes }); err != nil {
		return err
	}
	return writeFile(dir, PeriodStatsFile, experimentID, ids, func(i int) []PeriodStat { return data[i].Periods })
}

func normalizedFills(rows []Fill) []Fill {
	out := rows[:0:0]
	for _, r := range rows {
		if r.Side == "" {
			continue
		}
		r.Side = strings.ToLower(r.Side)
		out = append(out, r)
	}
	return out
}

// writeFile writes one file: for each instance in order, its rows as one row group. Instances
// without rows get no group and no footer entry.
func writeFile[T any](dir, name, experimentID string, ids []string, rowsOf func(i int) []T) (err error) {
	groups := make([][]T, len(ids))
	index := make(map[string]int, len(ids))
	for i, id := range ids {
		if rows := rowsOf(i); len(rows) > 0 {
			index[id] = len(index)
			groups[i] = rows
		}
	}
	idx, err := json.Marshal(index)
	if err != nil {
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
	w := parquet.NewGenericWriter[T](tmp,
		parquet.KeyValueMetadata(KeySchemaVersion, fmt.Sprint(SchemaVersion)),
		parquet.KeyValueMetadata(KeyExperimentID, experimentID),
		parquet.KeyValueMetadata(KeyInstances, string(idx)),
	)
	for _, rows := range groups {
		if len(rows) == 0 {
			continue
		}
		if _, err = w.Write(rows); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
		if err = w.Flush(); err != nil { // ends the row group of this instance
			return fmt.Errorf("flush %s: %w", name, err)
		}
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
