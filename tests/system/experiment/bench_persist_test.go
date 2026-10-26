package experiment_test

import (
	"context"
	"math"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MaxBernstetter/dagobert/src/backtest"
	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
)

// TestBenchPersistGrid measures one RunExperiment over a large ma-cross grid. It is a manual
// harness, skipped unless BENCH_INSTANCES is set. With DATABASE_URL set the run goes through
// Postgres (the grid is mirrored into it; the database must hold db/sqlc/schema.sql), else SQLite.
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
	dbURL := os.Getenv("DATABASE_URL")
	t.Setenv("DATABASE_URL", "")
	dataDir, cat, rec := gridExperiment(t, n)
	if dbURL != "" {
		t.Setenv("DATABASE_URL", dbURL)
		mirrorToPostgres(t, dbURL, rec)
	}
	start := time.Now()
	if err := backtest.RunExperiment(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	completed := -1
	if dbURL == "" {
		got, err := cat.GetExperiment(rec.ID)
		if err != nil {
			t.Fatal(err)
		}
		completed = countStatus(got)[store.StatusCompleted]
	}
	t.Logf("BENCH instances=%d workers=%s postgres=%v wall=%s completed(sqlite)=%d",
		n, os.Getenv("EXPERIMENT_SIM_WORKERS"), dbURL != "", elapsed.Round(time.Millisecond), completed)
}

func mirrorToPostgres(t *testing.T, dbURL string, rec store.ExperimentRecord) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	q := experimentdb.New(pool)
	eid := uuid.MustParse(rec.ID)
	s := rec.Settings
	if _, err := q.InsertExperiment(ctx, experimentdb.InsertExperimentParams{
		ID: eid, Name: "bench", Status: store.StatusDraft, Provider: s.Provider, Base: s.Base, Quote: s.Quote,
		FromMs: s.FromMs, ToMs: s.ToMs, InitialUsdc: s.InitialUSDC, InitialUsdt: s.InitialUSDT,
		TargetWeightPct: s.TargetWeightPct, MinOrderQuote: s.MinOrderQuote, Algo: "v1",
		CreatedAtMs: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatal(err)
	}
	for _, inst := range rec.Instances {
		if _, err := q.InsertInstance(ctx, experimentdb.InsertInstanceParams{
			ID: uuid.MustParse(inst.ID), ExperimentID: eid, Label: inst.Label, Playbook: inst.Playbook,
			Symbol: inst.Symbol, ConfigHash: inst.ConfigHash, PlaybookKey: inst.PlaybookKey,
			WorkspaceRelPath: inst.WorkspaceRelPath, Status: store.StatusDraft,
			CfgFastHr: pgtype.Int4{Int32: 3, Valid: true}, CfgSlowHr: pgtype.Int4{Int32: 12, Valid: true},
		}); err != nil {
			t.Fatal(err)
		}
	}
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
