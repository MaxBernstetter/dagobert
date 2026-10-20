package lake

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/parquet-go/parquet-go"

	"github.com/MaxBernstetter/trading-copilot/services/process/internal/bars"
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

// Read loads 1h candles. A missing file is ErrMissing. A file that does not
// have the required columns is ErrSchema. Other failures are returned after
// three attempts.
func Read(path string) ([]bars.Candle, error) {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		candles, err := readOnce(path)
		if err == nil || errors.Is(err, ErrMissing) || errors.Is(err, ErrSchema) {
			return candles, err
		}
		last = err
	}
	return nil, last
}

func readOnce(path string) (candles []bars.Candle, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("parquet read: %v", recovered)
		}
	}()

	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrMissing
		}
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	pf, err := parquet.OpenFile(file, info.Size())
	if err != nil {
		return nil, err
	}
	if !hasColumns(pf.Schema(), requiredColumns) {
		return nil, ErrSchema
	}

	reader := parquet.NewGenericReader[row](pf)
	defer reader.Close()

	buf := make([]row, 256)
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
				return candles, nil
			}
			return nil, readErr
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
