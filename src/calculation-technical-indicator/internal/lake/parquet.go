package lake

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/parquet-go/parquet-go"

	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/bars"
	"github.com/MaxBernstetter/dagobert/src/internal/lakeobject"
	"github.com/MaxBernstetter/dagobert/src/internal/parquetmeta"
	"github.com/MaxBernstetter/dagobert/src/shared"
)

var (
	// ErrMissing is a parquet file that is not on disk.
	ErrMissing = errors.New("parquet missing")
	// ErrSchema is a parquet file without the columns this job reads.
	ErrSchema = errors.New("parquet schema")
)

var requiredColumns = []string{
	"open_time",
	"open",
	"high",
	"low",
	"close",
	"volume",
	"close_time",
}

type row struct {
	OpenTime  int64   `parquet:"open_time"`
	Open      float64 `parquet:"open"`
	High      float64 `parquet:"high"`
	Low       float64 `parquet:"low"`
	Close     float64 `parquet:"close"`
	Volume    float64 `parquet:"volume"`
	CloseTime int64   `parquet:"close_time"`
}

// RawSeries is the candle stream plus lake metadata from the Parquet footer.
type RawSeries struct {
	Candles    []bars.Candle
	IntervalMS int64
	DataSource shared.DataSource
}

// Read loads candles from one or more Hive partition files (e.g. year=*).
// A market with no files is ErrMissing.
func Read(paths ...string) (RawSeries, error) {
	if len(paths) == 0 {
		return RawSeries{}, ErrMissing
	}
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		series, err := readMany(paths)
		if err == nil || errors.Is(err, ErrMissing) || errors.Is(err, ErrSchema) {
			return series, err
		}
		last = err
	}
	return RawSeries{}, last
}

func readMany(paths []string) (RawSeries, error) {
	var all []bars.Candle
	var intervalMS int64
	source := shared.DefaultDataSource()
	found := false
	for _, path := range paths {
		part, err := readOnce(path)
		if errors.Is(err, ErrMissing) {
			continue
		}
		if err != nil {
			return RawSeries{}, err
		}
		found = true
		all = append(all, part.Candles...)
		if part.IntervalMS > 0 {
			intervalMS = part.IntervalMS
		}
		if part.DataSource != 0 {
			source = part.DataSource
		}
	}
	if !found {
		return RawSeries{}, ErrMissing
	}
	sort.Slice(all, func(i, j int) bool { return all[i].OpenTime < all[j].OpenTime })
	if intervalMS <= 0 {
		opens := make([]int64, len(all))
		for i, c := range all {
			opens[i] = c.OpenTime
		}
		intervalMS = parquetmeta.InferIntervalMS(opens)
	}
	return RawSeries{Candles: all, IntervalMS: intervalMS, DataSource: source}, nil
}

func readOnce(path string) (series RawSeries, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("parquet read: %v", recovered)
		}
	}()

	file, size, err := lakeobject.OpenAbs(context.Background(), path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return RawSeries{}, ErrMissing
		}
		return RawSeries{}, err
	}
	defer file.Close()

	pf, err := parquet.OpenFile(file.File, size)
	if err != nil {
		return RawSeries{}, err
	}
	if !hasColumns(pf.Schema(), requiredColumns) {
		return RawSeries{}, ErrSchema
	}

	meta, _ := parquetmeta.FromFile(pf)
	source := shared.DefaultDataSource()
	if meta.DataSource != 0 {
		source = meta.DataSource
	}

	reader := parquet.NewGenericReader[row](pf)
	defer reader.Close()

	buf := make([]row, 256)
	var candles []bars.Candle
	for {
		n, readErr := reader.Read(buf)
		for _, item := range buf[:n] {
			candles = append(candles, bars.Candle{
				OpenTime:  item.OpenTime,
				Open:      item.Open,
				High:      item.High,
				Low:       item.Low,
				Close:     item.Close,
				Volume:    item.Volume,
				CloseTime: item.CloseTime,
			})
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				interval := meta.IntervalMS
				if interval <= 0 {
					opens := make([]int64, len(candles))
					for i, c := range candles {
						opens[i] = c.OpenTime
					}
					interval = parquetmeta.InferIntervalMS(opens)
				}
				return RawSeries{Candles: candles, IntervalMS: interval, DataSource: source}, nil
			}
			return RawSeries{}, readErr
		}
	}
}

func hasColumns(schema *parquet.Schema, names []string) bool {
	found := make(map[string]struct{}, len(names))
	for _, path := range schema.Columns() {
		if len(path) == 0 {
			continue
		}
		found[path[len(path)-1]] = struct{}{}
	}
	for _, name := range names {
		if _, ok := found[name]; !ok {
			return false
		}
	}
	return true
}
