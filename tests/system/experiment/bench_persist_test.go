package experiment_test

import (
	"math"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/MaxBernstetter/dagobert/src/backtest"
	"github.com/MaxBernstetter/dagobert/src/backtest/store"
)

// TestBenchPersistGrid measures one RunExperiment over a large ma-cross grid. It is a manual
// harness, skipped unless BENCH_INSTANCES is set. The grid is created in the scratch Postgres
// (POSTGRES_TEST_URL), the same catalog the run writes to.
//
//	BENCH_INSTANCES=3000 EXPERIMENT_SIM_WORKERS=22 go test ./tests/system/experiment -run BenchPersistGrid -v
//
// BENCH_HOURS sets the lake length in hourly bars (default 240). BENCH_WALK=1 swaps the default
// zigzag fixture (one fill pair per few bars, far more trades than real data) for a seeded random walk.
func TestBenchPersistGrid(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("BENCH_INSTANCES"))
	if n < 1 {
		t.Skip("BENCH_INSTANCES not set")
	}
	dataDir, url, rec := gridExperiment(t, n)
	start := time.Now()
	if err := backtest.RunExperiment(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	got := loadExperimentPG(t, url, rec.ID)
	t.Logf("BENCH instances=%d workers=%s wall=%s completed=%d",
		n, os.Getenv("EXPERIMENT_SIM_WORKERS"), elapsed.Round(time.Millisecond), countStatus(got)[store.StatusCompleted])
}

// writeWalkParquet writes endMs of hourly bars as a seeded geometric random walk (seed 110).
func writeWalkParquet(t *testing.T, dataDir string, endMs int64) {
	t.Helper()
	rng := rand.New(rand.NewSource(110))
	hours := int(endMs / hour)
	rows := make([]hourCandle, 0, hours)
	price := 100.0
	for i := 0; i < hours; i++ {
		price *= math.Exp(0.004 * rng.NormFloat64())
		ot := int64(i) * hour
		rows = append(rows, hourCandle{
			OpenTime: ot, Open: price, High: price * 1.002, Low: price * 0.998,
			Close: price, Volume: 10, CloseTime: ot + hour - 1,
		})
	}
	writeRows(t, dataDir, rows)
}
