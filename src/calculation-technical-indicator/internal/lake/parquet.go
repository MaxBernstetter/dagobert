package lake

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/parquet-go/parquet-go"

	"github.com/MaxBernstetter/trading-copilot/src/internal/parquetmeta"
	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/bars"
	"github.com/MaxBernstetter/trading-copilot/src/shared"
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

// RawSeries is the candle stream plus lake metadata from the parquet sidecar.
type RawSeries struct {
	Candles    []bars.Candle
	IntervalMS int64
	DataSource shared.DataSource
}

// Read loads candles and sidecar metadata. A missing file is ErrMissing.
func Read(path string) (RawSeries, error) {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		series, err := readOnce(path)
		if err == nil || errors.Is(err, ErrMissing) || errors.Is(err, ErrSchema) {
			return series, err
		}
		last = err
	}
	return RawSeries{}, last
}

func readOnce(path string) (series RawSeries, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("parquet read: %v", recovered)
		}
	}()

	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return RawSeries{}, ErrMissing
		}
		return RawSeries{}, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return RawSeries{}, err
	}
	pf, err := parquet.OpenFile(file, info.Size())
	if err != nil {
		return RawSeries{}, err
	}
	if !hasColumns(pf.Schema(), requiredColumns) {
		return RawSeries{}, ErrSchema
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
				interval, source, ierr := resolveLakeMeta(path, candles)
				if ierr != nil {
					return RawSeries{}, ierr
				}
				return RawSeries{Candles: candles, IntervalMS: interval, DataSource: source}, nil
			}
			return RawSeries{}, readErr
		}
	}
}

func resolveLakeMeta(parquetPath string, candles []bars.Candle) (intervalMS int64, source shared.DataSource, err error) {
	meta, ok, err := parquetmeta.Read(parquetPath)
	if err != nil {
		return 0, 0, err
	}
	source = shared.DefaultDataSource()
	if ok && meta.DataSource != 0 {
		source = meta.DataSource
	}
	if ok && meta.IntervalMS > 0 {
		return meta.IntervalMS, source, nil
	}
	sort.Slice(candles, func(i, j int) bool { return candles[i].OpenTime < candles[j].OpenTime })
	opens := make([]int64, len(candles))
	for i, c := range candles {
		opens[i] = c.OpenTime
	}
	return parquetmeta.InferIntervalMS(opens), source, nil
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
