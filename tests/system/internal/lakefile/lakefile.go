// Package lakefile writes candle Parquet files for system tests. It encodes the
// lake schema on its own (DECIMAL(18,8) INT64 prices, DECIMAL(28,8)
// FIXED_LEN_BYTE_ARRAY(12) volume) so the tests do not depend on the service
// packages that read the files.
package lakefile

import (
	"encoding/binary"
	"math"

	"github.com/parquet-go/parquet-go"
)

// Candle is one hour with float values; they are rounded to 8 decimals.
type Candle struct {
	OpenTime  int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	CloseTime int64
}

type diskRow struct {
	OpenTime  int64    `parquet:"open_time,delta,snappy"`
	Open      int64    `parquet:"open,decimal(8:18),delta,snappy"`
	High      int64    `parquet:"high,decimal(8:18),delta,snappy"`
	Low       int64    `parquet:"low,decimal(8:18),delta,snappy"`
	Close     int64    `parquet:"close,decimal(8:18),delta,snappy"`
	Volume    [12]byte `parquet:"volume,decimal(8:28),snappy"`
	CloseTime int64    `parquet:"close_time,delta,snappy"`
}

func unscaled(v float64) int64 { return int64(math.Round(v * 1e8)) }

// Write stores rows at path in the lake schema.
func Write(path string, rows []Candle) error {
	disk := make([]diskRow, len(rows))
	for i, r := range rows {
		d := diskRow{
			OpenTime: r.OpenTime, CloseTime: r.CloseTime,
			Open: unscaled(r.Open), High: unscaled(r.High), Low: unscaled(r.Low), Close: unscaled(r.Close),
		}
		v := unscaled(r.Volume)
		fill := byte(0)
		if v < 0 {
			fill = 0xff
		}
		for j := 0; j < 4; j++ {
			d.Volume[j] = fill
		}
		binary.BigEndian.PutUint64(d.Volume[4:], uint64(v))
		disk[i] = d
	}
	return parquet.WriteFile(path, disk,
		parquet.KeyValueMetadata("trading_copilot.schema_version", "2"),
		parquet.KeyValueMetadata("trading_copilot.interval_ms", "3600000"),
		parquet.KeyValueMetadata("trading_copilot.data_source", "binance"))
}
