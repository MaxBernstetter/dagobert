package shared

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// CandlesParquetFile is the fixed object name inside a Hive partition directory.
	CandlesParquetFile = "candles.parquet"
)

// ValidatePartitionValue rejects empty values and characters that break Hive path keys.
func ValidatePartitionValue(v string) error {
	if v == "" {
		return fmt.Errorf("empty partition value")
	}
	for _, r := range v {
		if r == '/' || r == '=' || r == 0 {
			return fmt.Errorf("invalid partition value %q", v)
		}
	}
	return nil
}

// YearUTC returns the UTC calendar year for an open_time millisecond.
func YearUTC(openTimeMs int64) int {
	return time.UnixMilli(openTimeMs).UTC().Year()
}

// YearsSpanning returns inclusive UTC years covering [fromMs, toMs).
// When toMs <= fromMs the result is empty.
func YearsSpanning(fromMs, toMs int64) []int {
	if toMs <= fromMs {
		return nil
	}
	y0 := YearUTC(fromMs)
	y1 := YearUTC(toMs - 1)
	if y1 < y0 {
		return nil
	}
	out := make([]int, 0, y1-y0+1)
	for y := y0; y <= y1; y++ {
		out = append(out, y)
	}
	return out
}

// HiveMarketDir returns DATA_DIR/parquet/provider={id}/base={base}/quote={quote}.
func HiveMarketDir(dataDir, provider, base, quote string) (string, error) {
	if err := ValidatePartitionValue(provider); err != nil {
		return "", fmt.Errorf("provider: %w", err)
	}
	m, err := NormalizeMarket(base, quote)
	if err != nil {
		return "", err
	}
	return filepath.Join(
		dataDir,
		DataParquetSubdir,
		"provider="+provider,
		"base="+m.Base,
		"quote="+m.Quote,
	), nil
}

// HivePartitionDir returns .../year={YYYY} under the market directory.
func HivePartitionDir(dataDir, provider, base, quote string, year int) (string, error) {
	market, err := HiveMarketDir(dataDir, provider, base, quote)
	if err != nil {
		return "", err
	}
	if year < 1970 || year > 9999 {
		return "", fmt.Errorf("invalid year %d", year)
	}
	return filepath.Join(market, "year="+strconv.Itoa(year)), nil
}

// ParquetPath returns the Hive candles parquet path for provider+base+quote+year.
func ParquetPath(dataDir, provider, base, quote string, year int) (string, error) {
	dir, err := HivePartitionDir(dataDir, provider, base, quote, year)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, CandlesParquetFile), nil
}

// EnsureProviderMarketLayout creates the market Hive directory (years created on write).
func EnsureProviderMarketLayout(dataDir, provider, base, quote string) error {
	dir, err := HiveMarketDir(dataDir, provider, base, quote)
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}

// ListCandleParquetPaths lists year=*/candles.parquet under the market directory.
func ListCandleParquetPaths(dataDir, provider, base, quote string) ([]string, error) {
	market, err := HiveMarketDir(dataDir, provider, base, quote)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(market)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var years []int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "year=") {
			continue
		}
		y, err := strconv.Atoi(strings.TrimPrefix(name, "year="))
		if err != nil || y < 1970 || y > 9999 {
			continue
		}
		years = append(years, y)
	}
	sort.Ints(years)
	out := make([]string, 0, len(years))
	for _, y := range years {
		out = append(out, filepath.Join(market, "year="+strconv.Itoa(y), CandlesParquetFile))
	}
	return out, nil
}

// ListCandleParquetPathsInRange lists candle files for years overlapping [fromMs, toMs).
func ListCandleParquetPathsInRange(dataDir, provider, base, quote string, fromMs, toMs int64) ([]string, error) {
	years := YearsSpanning(fromMs, toMs)
	if len(years) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(years))
	for _, y := range years {
		path, err := ParquetPath(dataDir, provider, base, quote, y)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		out = append(out, path)
	}
	return out, nil
}
