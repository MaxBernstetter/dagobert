package job

import (
	"context"
	"log"
	"math"
	"os"
	"time"
)

// Run executes one ingest. It returns a process status of 0, 1, or 2 and a detail code.
// The caller prints DETAIL. Diagnostics go to stderr.
func Run(args []string, getenv func(string) string) (code, detail int) {
	cfg, err := parseConfig(args, getenv)
	if err != nil {
		log.Printf("invalid input: %v", err)
		return 1, 10
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Printf("data dir: %v", err)
		return 1, 40
	}
	st, err := openStore(cfg.DataDir)
	if err != nil {
		log.Printf("sqlite: %v", err)
		return 1, 40
	}
	defer st.close()

	stored, err := st.list(cfg.Symbol, cfg.From, cfg.To)
	if err != nil {
		log.Printf("list stored: %v", err)
		return 1, 40
	}
	path := parquetPath(cfg.DataDir, cfg.Symbol)
	existing, exists, err := readCandles(path)
	if err != nil {
		log.Printf("parquet: %v", err)
		return 1, 40
	}
	if !exists && len(stored) > 0 {
		log.Printf("parquet missing for stored hours")
		return 1, 40
	}
	have := make(map[int64]struct{}, len(existing))
	for _, c := range existing {
		have[c.OpenTime] = struct{}{}
	}
	for _, t := range stored {
		if _, ok := have[t]; !ok {
			log.Printf("stored open_time %d missing from parquet", t)
			return 1, 40
		}
	}

	expected, ok := hourOpens(cfg.From, cfg.To)
	if !ok {
		log.Printf("hour grid overflow")
		return 1, 10
	}
	now := time.Now().UnixMilli()
	for _, open := range expected {
		if stillOpen(open, now) {
			log.Printf("hour %d still open", open)
			return 2, 20
		}
	}

	storedSet := make(map[int64]struct{}, len(stored))
	for _, t := range stored {
		storedSet[t] = struct{}{}
	}
	var missing []int64
	for _, open := range expected {
		if _, ok := storedSet[open]; !ok {
			missing = append(missing, open)
		}
	}
	if len(missing) == 0 {
		return 2, 1
	}

	f, err := newFetcher(cfg)
	if err != nil {
		log.Printf("client: %v", err)
		return 1, 10
	}
	rows, err := f.fetch(context.Background(), windows(missing, cfg.To))
	if err != nil {
		log.Printf("binance: %v", err)
		return 1, 30
	}

	want := make(map[int64]struct{}, len(missing))
	for _, t := range missing {
		want[t] = struct{}{}
	}
	var fresh []Candle
	seen := make(map[int64]struct{}, len(missing))
	for _, c := range rows {
		if c.OpenTime < cfg.From || c.OpenTime >= cfg.To || c.OpenTime%hourMS != 0 {
			continue
		}
		if c.CloseTime > now {
			continue
		}
		if _, ok := want[c.OpenTime]; !ok {
			continue
		}
		if _, ok := seen[c.OpenTime]; ok {
			continue
		}
		seen[c.OpenTime] = struct{}{}
		fresh = append(fresh, c)
	}
	if len(fresh) == 0 {
		log.Printf("no closed missing hours in the response")
		return 1, 30
	}

	merged, newTimes := mergeCandles(existing, fresh)
	if err := writeCandles(path, merged); err != nil {
		log.Printf("parquet write: %v", err)
		return 1, 40
	}
	if err := st.insert(cfg.Symbol, newTimes); err != nil {
		log.Printf("sqlite insert: %v", err)
		return 1, 40
	}
	return 0, 0
}

func stillOpen(open, now int64) bool {
	if open > math.MaxInt64-hourMS+1 {
		return true
	}
	return open+hourMS-1 > now
}
