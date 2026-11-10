package tiparquet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/parquet-go/parquet-go"

	"github.com/MaxBernstetter/dagobert/src/internal/lakeobject"
	"github.com/MaxBernstetter/dagobert/src/shared"
)

const barsFile = "bars.parquet"

// Row is one TI cache bar (v1 keeps results_json for indicator graph).
type Row struct {
	BarOpen     int64   `parquet:"bar_open"`
	Open        float64 `parquet:"open"`
	High        float64 `parquet:"high"`
	Low         float64 `parquet:"low"`
	Close       float64 `parquet:"close"`
	Volume      float64 `parquet:"volume"`
	CloseTime   int64   `parquet:"close_time"`
	ResultsJSON string  `parquet:"results_json"`
}

// HiveDir returns DATA_DIR/parquet/ti/provider=/base=/quote=/algo=/year=.
func HiveDir(dataDir, provider, base, quote, algo string, year int) (string, error) {
	if err := shared.ValidatePartitionValue(provider); err != nil {
		return "", err
	}
	m, err := shared.NormalizeMarket(base, quote)
	if err != nil {
		return "", err
	}
	if algo == "" {
		algo = "v1"
	}
	if err := shared.ValidatePartitionValue(algo); err != nil {
		return "", err
	}
	return filepath.Join(
		dataDir, "parquet", "ti",
		"provider="+provider,
		"base="+m.Base,
		"quote="+m.Quote,
		"algo="+algo,
		fmt.Sprintf("year=%d", year),
	), nil
}

// Path returns the bars.parquet path for a year partition.
func Path(dataDir, provider, base, quote, algo string, year int) (string, error) {
	dir, err := HiveDir(dataDir, provider, base, quote, algo, year)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, barsFile), nil
}

// WriteYear writes (replaces) one year partition.
func WriteYear(dataDir, provider, base, quote, algo string, year int, rows []Row) error {
	if len(rows) == 0 {
		return nil
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].BarOpen < rows[j].BarOpen })
	path, err := Path(dataDir, provider, base, quote, algo, year)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	_ = os.Remove(tmp)
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := parquet.NewGenericWriter[Row](f)
	if _, err := w.Write(rows); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := w.Close(); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return lakeobject.Publish(context.Background(), dataDir, path)
}

// ReadYear loads one year partition; missing file returns nil, nil.
func ReadYear(dataDir, provider, base, quote, algo string, year int) ([]Row, error) {
	path, err := Path(dataDir, provider, base, quote, algo, year)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	ok, err := lakeobject.Exists(ctx, dataDir, path)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	f, size, err := lakeobject.Open(ctx, dataDir, path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	pf, err := parquet.OpenFile(f.File, size)
	if err != nil {
		return nil, err
	}
	reader := parquet.NewGenericReader[Row](pf)
	defer reader.Close()
	var out []Row
	buf := make([]Row, 256)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			out = append(out, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	return out, nil
}

// LoadRange loads TI rows for [from,to) across year partitions.
// ok is false when any required year partition is missing or range is not fully covered.
func LoadRange(dataDir, provider, base, quote, algo string, from, to, aggMS int64) (rows []Row, ok bool, err error) {
	years := shared.YearsSpanning(from, to)
	if len(years) == 0 {
		return nil, false, nil
	}
	var all []Row
	for _, y := range years {
		part, err := ReadYear(dataDir, provider, base, quote, algo, y)
		if err != nil {
			return nil, false, err
		}
		if part == nil {
			return nil, false, nil
		}
		all = append(all, part...)
	}
	if len(all) == 0 {
		return nil, false, nil
	}
	sort.Slice(all, func(i, j int) bool { return all[i].BarOpen < all[j].BarOpen })
	filtered := make([]Row, 0, len(all))
	for _, r := range all {
		closeMS := r.BarOpen + aggMS
		if r.BarOpen >= from && closeMS <= to {
			filtered = append(filtered, r)
		}
	}
	if len(filtered) == 0 {
		return nil, false, nil
	}
	// Coverage: the cached bars must start within one bucket of from and end within one bucket of to
	// (a trailing partial bucket is allowed). A cache that ends earlier is a miss, so the caller rebuilds.
	if filtered[0].BarOpen > from+aggMS {
		return nil, false, nil
	}
	if last := filtered[len(filtered)-1]; to-(last.BarOpen+aggMS) >= aggMS {
		return nil, false, nil
	}
	return filtered, true, nil
}

// WriteRange partitions rows by UTC year and writes each year file.
func WriteRange(dataDir, provider, base, quote, algo string, rows []Row) error {
	byYear := map[int][]Row{}
	for _, r := range rows {
		y := shared.YearUTC(r.BarOpen)
		byYear[y] = append(byYear[y], r)
	}
	for y, list := range byYear {
		if err := WriteYear(dataDir, provider, base, quote, algo, y, list); err != nil {
			return err
		}
	}
	return nil
}
