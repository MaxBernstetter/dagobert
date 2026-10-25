package parquetmeta

import (
	"fmt"
	"os"
	"strconv"

	"github.com/parquet-go/parquet-go"

	"github.com/MaxBernstetter/dagobert/src/shared"
)

// Footer key–value schema for lake candle files (Parquet FileMetaData).
const (
	SchemaVersion = 1

	KeySchemaVersion = "trading_copilot.schema_version"
	KeyIntervalMS    = "trading_copilot.interval_ms"
	KeyDataSource    = "trading_copilot.data_source"
)

// Meta is the lake candle file metadata written into the Parquet footer.
type Meta struct {
	SchemaVersion int
	IntervalMS    int64
	DataSource    shared.DataSource
}

// WriterOptions returns parquet-go options that embed Meta in the file footer.
func WriterOptions(m Meta) []parquet.WriterOption {
	if m.SchemaVersion == 0 {
		m.SchemaVersion = SchemaVersion
	}
	return []parquet.WriterOption{
		parquet.KeyValueMetadata(KeySchemaVersion, strconv.Itoa(m.SchemaVersion)),
		parquet.KeyValueMetadata(KeyIntervalMS, strconv.FormatInt(m.IntervalMS, 10)),
		parquet.KeyValueMetadata(KeyDataSource, m.DataSource.String()),
	}
}

// FromFile reads Meta from an open Parquet file's key–value metadata.
func FromFile(f *parquet.File) (Meta, bool) {
	if f == nil {
		return Meta{}, false
	}
	verStr, okVer := f.Lookup(KeySchemaVersion)
	intervalStr, okInterval := f.Lookup(KeyIntervalMS)
	sourceStr, okSource := f.Lookup(KeyDataSource)
	if !okVer && !okInterval && !okSource {
		return Meta{}, false
	}
	var m Meta
	if okVer {
		if v, err := strconv.Atoi(verStr); err == nil {
			m.SchemaVersion = v
		}
	}
	if okInterval {
		if v, err := strconv.ParseInt(intervalStr, 10, 64); err == nil {
			m.IntervalMS = v
		}
	}
	if okSource {
		if ds, ok := shared.ParseDataSource(sourceStr); ok {
			m.DataSource = ds
		}
	}
	return m, true
}

// Read opens path and loads footer Meta when present.
func Read(parquetPath string) (Meta, bool, error) {
	file, err := os.Open(parquetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Meta{}, false, nil
		}
		return Meta{}, false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Meta{}, false, err
	}
	pf, err := parquet.OpenFile(file, info.Size())
	if err != nil {
		return Meta{}, false, fmt.Errorf("parquet open: %w", err)
	}
	m, ok := FromFile(pf)
	return m, ok, nil
}

// InferIntervalMS estimates the candle period from consecutive open times.
func InferIntervalMS(openTimes []int64) int64 {
	if len(openTimes) < 2 {
		return 0
	}
	minDelta := openTimes[1] - openTimes[0]
	if minDelta <= 0 {
		return 0
	}
	for i := 2; i < len(openTimes); i++ {
		d := openTimes[i] - openTimes[i-1]
		if d > 0 && d < minDelta {
			minDelta = d
		}
	}
	return minDelta
}
