package job

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/parquet-go/parquet-go"

	"github.com/MaxBernstetter/trading-copilot/src/internal/parquetmeta"
	"github.com/MaxBernstetter/trading-copilot/src/shared"
)

// Candle is the on-disk row. Column order is the struct field order.
type Candle struct {
	OpenTime  int64  `parquet:"open_time"`
	Open      string `parquet:"open"`
	High      string `parquet:"high"`
	Low       string `parquet:"low"`
	Close     string `parquet:"close"`
	Volume    string `parquet:"volume"`
	CloseTime int64  `parquet:"close_time"`
}

func parquetPath(dataDir, symbol string) string {
	return filepath.Join(dataDir, "parquet", symbol+".parquet")
}

func readCandles(path string) (rows []Candle, exists bool, err error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	rows, err = parquet.ReadFile[Candle](path)
	if err != nil {
		return nil, true, err
	}
	return rows, true, nil
}

func writeCandles(path string, rows []Candle) error {
	sort.Slice(rows, func(i, j int) bool { return rows[i].OpenTime < rows[j].OpenTime })
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			os.Remove(tmpName)
		}
	}()
	if err := parquet.Write(tmp, rows); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	if err := parquetmeta.Write(path, parquetmeta.Meta{
		IntervalMS: hourMS,
		DataSource: shared.DataSourceBinance,
	}); err != nil {
		return err
	}
	ok = true
	return nil
}

func mergeCandles(existing, fresh []Candle) (merged []Candle, newTimes []int64) {
	by := make(map[int64]Candle, len(existing)+len(fresh))
	for _, c := range existing {
		by[c.OpenTime] = c
	}
	seen := make(map[int64]struct{}, len(fresh))
	for _, c := range fresh {
		by[c.OpenTime] = c
		if _, ok := seen[c.OpenTime]; ok {
			continue
		}
		seen[c.OpenTime] = struct{}{}
		newTimes = append(newTimes, c.OpenTime)
	}
	sort.Slice(newTimes, func(i, j int) bool { return newTimes[i] < newTimes[j] })
	merged = make([]Candle, 0, len(by))
	for _, c := range by {
		merged = append(merged, c)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].OpenTime < merged[j].OpenTime })
	return merged, newTimes
}
