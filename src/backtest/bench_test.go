package backtest

import (
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"testing"
	"time"

	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	signalrunner "github.com/MaxBernstetter/dagobert/src/calculation-signal/runner"
	tirunner "github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/runner"
	"github.com/MaxBernstetter/dagobert/src/internal/kpi"
	"github.com/MaxBernstetter/dagobert/src/internal/signalstore"
)

// Go counterpart of the C++ benchmark (docs/development/05-cpp-benchmark.md).
// It runs the experiment path (prepare = Parquet + indicators, simulate =
// fast pipeline + report) in process on the lake named by BENCH_LAKE_DIR
// (written by `go run ./src/calculation/testdata/genlakefixture -bench <dir>`).
// Run one sub-benchmark per process so ru_maxrss is attributable.

type benchCase struct {
	name     string
	playbook string
	config   string
}

var benchCases = []benchCase{
	{"sma-cross", "ma-cross", `{"fastHr":12,"slowHr":48}`},
	{"signal-consensus", "signal-consensus", ``},
}

type benchPrepared struct {
	win    *kpi.Window
	closes []int64
}

func benchPrepare(b *testing.B, lake string, from, to int64) benchPrepared {
	b.Helper()
	dataDir := b.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "parquet"), 0o755); err != nil {
		b.Fatal(err)
	}
	// Share the raw lake, keep the indicator cache (parquet/ti) per iteration.
	src, err := filepath.Abs(filepath.Join(lake, "parquet", "provider=binance"))
	if err != nil {
		b.Fatal(err)
	}
	if err := os.Symlink(src, filepath.Join(dataDir, "parquet", "provider=binance")); err != nil {
		b.Fatal(err)
	}
	settings := store.DefaultExperimentSettings()
	settings.FromMs, settings.ToMs = from, to
	applyExperimentEnv(dataDir, settings)

	b.StartTimer()
	rows, err := tirunner.BuildKPIRange(dataDir, "BTCUSDT", from, to, time.Now().UnixMilli())
	if err != nil {
		b.Fatal(err)
	}
	bodies := make(map[int64]string, len(rows))
	closes := make([]int64, 0, len(rows))
	aggMS := signalstore.AggregationDurationMS()
	for _, row := range rows {
		bodies[row.BarOpen] = row.Body
		closes = append(closes, row.BarOpen+aggMS)
	}
	win, err := kpi.LoadWindowBodies(bodies)
	if err != nil {
		b.Fatal(err)
	}
	return benchPrepared{win: win, closes: closes}
}

func benchSimulate(b *testing.B, c benchCase, p benchPrepared, from, to int64) {
	b.Helper()
	ws := b.TempDir()
	settings := store.DefaultExperimentSettings()
	if err := writeExperimentPaperWallet(ws, settings, "BTCUSDT"); err != nil {
		b.Fatal(err)
	}
	b.StartTimer()
	opts := &signalrunner.PipelineOpts{StrategyConfig: []byte(c.config), Slim: true, SkipSampler: true}
	if c.config == "" {
		opts.StrategyConfig = nil
	}
	rep, err := signalrunner.FastSimulatedPipelineOpts(ws, p.win, c.playbook, "BTCUSDT", p.closes, opts, nil)
	if err != nil {
		b.Fatal(err)
	}
	if _, err := signalrunner.BuildAndWriteInstanceReport(ws, rep, c.playbook, "BTCUSDT", from, to,
		settings.InitialUSDC+settings.InitialUSDT, p.win, p.closes, signalstore.AggregationDurationMS()); err != nil {
		b.Fatal(err)
	}
}

func BenchmarkGoExperiment(b *testing.B) {
	lake := os.Getenv("BENCH_LAKE_DIR")
	if lake == "" {
		b.Skip("BENCH_LAKE_DIR not set")
	}
	_ = os.Setenv("VENUE_SIMULATE", "1")
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	to := time.Date(2025, 12, 30, 0, 0, 0, 0, time.UTC).UnixMilli()
	for _, c := range benchCases {
		for _, stage := range []string{"prepare", "simulate", "end_to_end"} {
			b.Run(c.name+"/"+stage, func(b *testing.B) {
				var prepared benchPrepared
				if stage == "simulate" {
					b.StopTimer()
					prepared = benchPrepare(b, lake, from, to)
				}
				durs := make([]float64, 0, b.N)
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					start := time.Time{}
					switch stage {
					case "prepare":
						start = time.Now()
						benchPrepare(b, lake, from, to)
					case "simulate":
						start = time.Now()
						benchSimulate(b, c, prepared, from, to)
					default:
						start = time.Now()
						p := benchPrepare(b, lake, from, to)
						benchSimulate(b, c, p, from, to)
					}
					durs = append(durs, float64(time.Since(start).Milliseconds()))
				}
				sort.Float64s(durs)
				b.ReportMetric(durs[len(durs)/2], "median_ms")
				b.ReportMetric(durs[(len(durs)*95+99)/100-1], "p95_ms")
				var ru syscall.Rusage
				_ = syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
				b.ReportMetric(float64(ru.Maxrss), "peak_rss_kb")
			})
		}
	}
}
