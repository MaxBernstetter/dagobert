package job

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"github.com/parquet-go/parquet-go"

	"github.com/MaxBernstetter/dagobert/src/ingest/provider"
	"github.com/MaxBernstetter/dagobert/src/internal/lakeobject"
	"github.com/MaxBernstetter/dagobert/src/internal/parquetmeta"
	"github.com/MaxBernstetter/dagobert/src/shared"
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

func toJobCandles(in []provider.Candle) []Candle {
	out := make([]Candle, len(in))
	for i, c := range in {
		out[i] = Candle{
			OpenTime:  c.OpenTime,
			Open:      c.Open,
			High:      c.High,
			Low:       c.Low,
			Close:     c.Close,
			Volume:    c.Volume,
			CloseTime: c.CloseTime,
		}
	}
	return out
}

func readCandles(dataDir, path string) (rows []Candle, exists bool, err error) {
	ctx := context.Background()
	ok, err := lakeobject.Exists(ctx, dataDir, path)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	f, size, err := lakeobject.Open(ctx, dataDir, path)
	if err != nil {
		return nil, true, err
	}
	defer f.Close()
	rows, err = parquet.Read[Candle](f.File, size)
	if err != nil {
		return nil, true, err
	}
	return rows, true, nil
}

func readMarketCandles(dataDir, providerID, base, quote string) ([]Candle, error) {
	paths, err := lakeobject.ListCandleParquetAbsPaths(context.Background(), dataDir, providerID, base, quote)
	if err != nil {
		return nil, err
	}
	var all []Candle
	for _, path := range paths {
		rows, exists, err := readCandles(dataDir, path)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		all = append(all, rows...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].OpenTime < all[j].OpenTime })
	return all, nil
}

func writeCandles(dataDir, path string, rows []Candle, source shared.DataSource) error {
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
	meta := parquetmeta.Meta{IntervalMS: hourMS, DataSource: source}
	if err := parquet.Write(tmp, rows, parquetmeta.WriterOptions(meta)...); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	ok = true
	_ = lakeobject.PutLocalFile(context.Background(), dataDir, path)
	return nil
}

// writeTouchedYearCandles rewrites only year partitions touched by fresh rows.
func writeTouchedYearCandles(dataDir, providerID, base, quote string, existing, fresh []Candle, source shared.DataSource) error {
	touched := map[int]struct{}{}
	for _, c := range fresh {
		touched[shared.YearUTC(c.OpenTime)] = struct{}{}
	}
	if len(touched) == 0 {
		return nil
	}
	byYear := map[int][]Candle{}
	for _, c := range existing {
		y := shared.YearUTC(c.OpenTime)
		if _, ok := touched[y]; ok {
			byYear[y] = append(byYear[y], c)
		}
	}
	for _, c := range fresh {
		y := shared.YearUTC(c.OpenTime)
		byYear[y] = append(byYear[y], c)
	}
	years := make([]int, 0, len(byYear))
	for y := range byYear {
		years = append(years, y)
	}
	sort.Ints(years)
	for _, y := range years {
		// Dedupe by open_time (fresh wins).
		byOpen := make(map[int64]Candle, len(byYear[y]))
		for _, c := range byYear[y] {
			byOpen[c.OpenTime] = c
		}
		rows := make([]Candle, 0, len(byOpen))
		for _, c := range byOpen {
			rows = append(rows, c)
		}
		path, err := shared.ParquetPath(dataDir, providerID, base, quote, y)
		if err != nil {
			return err
		}
		if err := writeCandles(dataDir, path, rows, source); err != nil {
			return err
		}
	}
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
	merged = make([]Candle, 0, len(by))
	for _, c := range by {
		merged = append(merged, c)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].OpenTime < merged[j].OpenTime })
	return merged, newTimes
}
