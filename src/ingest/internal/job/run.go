package job

import (
	"context"
	"errors"
	"log"
	"math"
	"os"
	"time"

	"github.com/MaxBernstetter/dagobert/src/ingest/internal/fetchclient"
	"github.com/MaxBernstetter/dagobert/src/internal/candlefile"
	"github.com/MaxBernstetter/dagobert/src/shared"
)

// Detail codes (printed as DETAIL=n by the ingest binary):
//
//	0  — wrote at least one hour (may also have recorded unavailable holes)
//	1  — abort: every closed hour already stored or unavailable (exit 2)
//	10 — invalid input
//	20 — range includes a still-open hour (exit 2)
//	30 — provider transport / retries exhausted
//	32 — no new candles; confirmed exchange holes recorded as unavailable
//	40 — store / parquet inconsistency
//	50 — a value has more than 8 decimals or does not fit the fixed-scale decimal columns
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
	if err := shared.EnsureDataLayout(cfg.DataDir); err != nil {
		log.Printf("data layout: %v", err)
		return 1, 40
	}
	if err := shared.EnsureProviderMarketLayout(cfg.DataDir, cfg.Provider, cfg.Base, cfg.Quote); err != nil {
		log.Printf("hive layout: %v", err)
		return 1, 40
	}

	ad, err := openAdapter(cfg)
	if err != nil {
		log.Printf("provider: %v", err)
		return 1, 10
	}

	st, err := openStore(cfg.DataDir)
	if err != nil {
		log.Printf("sqlite: %v", err)
		return 1, 40
	}
	defer st.close()

	if cfg.Force {
		if err := st.clearUnavailable(cfg.Provider, cfg.Base, cfg.Quote, cfg.From, cfg.To); err != nil {
			log.Printf("clear unavailable: %v", err)
			return 1, 40
		}
	}

	stored, err := st.list(cfg.Provider, cfg.Base, cfg.Quote, cfg.From, cfg.To)
	if err != nil {
		log.Printf("list stored: %v", err)
		return 1, 40
	}
	unavailable, err := st.listUnavailable(cfg.Provider, cfg.Base, cfg.Quote, cfg.From, cfg.To)
	if err != nil {
		log.Printf("list unavailable: %v", err)
		return 1, 40
	}
	existing, err := readMarketCandles(cfg.DataDir, cfg.Provider, cfg.Base, cfg.Quote)
	if err != nil {
		log.Printf("parquet: %v", err)
		return 1, 40
	}
	if len(existing) == 0 && len(stored) > 0 {
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
	unavailSet := make(map[int64]struct{}, len(unavailable))
	for _, t := range unavailable {
		unavailSet[t] = struct{}{}
	}
	var missing []int64
	for _, open := range expected {
		if cfg.Force {
			missing = append(missing, open)
			continue
		}
		if _, ok := have[open]; ok {
			continue
		}
		if _, ok := storedSet[open]; ok {
			continue
		}
		if _, ok := unavailSet[open]; ok {
			continue
		}
		missing = append(missing, open)
	}
	if len(missing) == 0 {
		return 2, 1
	}

	rows, err := fetchclient.FetchAll(context.Background(), ad, cfg.Base, cfg.Quote, missing, cfg.To, fetchclient.TransportConfig{
		MaxRetries:    cfg.MaxRetries,
		BackoffFactor: cfg.BackoffFactor,
		RetryWait:     cfg.RetryWait,
		MaxRetryWait:  cfg.MaxRetryWait,
		RateLimit:     cfg.RateLimit,
		Concurrency:   cfg.Concurrency,
	})
	if err != nil {
		log.Printf("%s: %v", ad.ID(), err)
		return 1, 30
	}
	freshProv := toJobCandles(rows)
	if err := validateCandles(freshProv); err != nil {
		log.Printf("candle value: %v", err)
		return 1, 50
	}

	want := make(map[int64]struct{}, len(missing))
	for _, t := range missing {
		want[t] = struct{}{}
	}
	var fresh []Candle
	seen := make(map[int64]struct{}, len(missing))
	for _, c := range freshProv {
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

	var holes []int64
	for _, t := range missing {
		if _, ok := seen[t]; ok {
			continue
		}
		holes = append(holes, t)
		log.Printf("unavailable hour %s (%s returned no closed candle)",
			time.UnixMilli(t).UTC().Format(time.RFC3339), ad.ID())
	}
	if len(holes) > 0 {
		if err := st.insertUnavailable(cfg.Provider, cfg.Base, cfg.Quote, holes, ad.EmptyReason(), now); err != nil {
			log.Printf("unavailable insert: %v", err)
			return 1, 40
		}
	}

	if len(fresh) == 0 {
		if len(holes) > 0 {
			return 0, 32
		}
		log.Printf("no closed missing hours in the response and no holes to record")
		return 1, 30
	}

	_, newTimes := mergeCandles(existing, fresh)
	if err := writeTouchedYearCandles(cfg.DataDir, cfg.Provider, cfg.Base, cfg.Quote, existing, fresh, ad.DataSource()); err != nil {
		log.Printf("parquet write: %v", err)
		var ve *candlefile.ValueError
		if errors.As(err, &ve) {
			return 1, 50
		}
		return 1, 40
	}
	if err := st.insert(cfg.Provider, cfg.Base, cfg.Quote, newTimes); err != nil {
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
