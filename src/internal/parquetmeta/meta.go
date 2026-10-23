package parquetmeta

import (
	"encoding/json"
	"os"

	"github.com/MaxBernstetter/trading-copilot/src/shared"
)

// Meta describes the candle stream beside a parquet file.
type Meta struct {
	IntervalMS int64            `json:"interval_ms"`
	DataSource shared.DataSource `json:"data_source"`
}

func sidecarPath(parquetPath string) string {
	return parquetPath + ".meta.json"
}

// Write records raw candle interval next to a parquet file.
func Write(parquetPath string, meta Meta) error {
	raw, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	tmp := sidecarPath(parquetPath) + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, sidecarPath(parquetPath))
}

// Read loads sidecar metadata when present.
func Read(parquetPath string) (Meta, bool, error) {
	path := sidecarPath(parquetPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Meta{}, false, nil
		}
		return Meta{}, false, err
	}
	var meta Meta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return Meta{}, true, err
	}
	return meta, true, nil
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
