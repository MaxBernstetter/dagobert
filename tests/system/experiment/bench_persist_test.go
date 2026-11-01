package experiment_test

import (
	"io/fs"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/parquet-go/parquet-go"

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
	load := hostLoad()
	start := time.Now()
	if err := backtest.RunExperiment(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	got := loadExperimentPG(t, url, rec.ID)
	t.Logf("BENCH instances=%d kpi_workers=%d cpp_processes=%d wall=%s completed=%d",
		n, backtest.ExperimentSimWorkers(), cppProcesses(), elapsed.Round(time.Millisecond), countStatus(got)[store.StatusCompleted])
	t.Logf("BENCH host loadavg before=%s after=%s cpus=%d", load, hostLoad(), runtime.NumCPU())
	logExperimentFiles(t, filepath.Join(dataDir, "experiments", rec.ID))
}

// cppProcesses is the effective C++ process count: EXPERIMENT_CPP_PROCESSES, else one per CPU.
func cppProcesses() int {
	if v, err := strconv.Atoi(os.Getenv("EXPERIMENT_CPP_PROCESSES")); err == nil && v >= 1 {
		return v
	}
	return runtime.NumCPU()
}

// hostLoad is /proc/loadavg (1, 5, 15 minute averages), or "n/a".
func hostLoad() string {
	raw, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return "n/a"
	}
	return strings.Join(strings.Fields(string(raw))[:3], " ")
}

// logExperimentFiles reports, per file name, the file count, the bytes and (for Parquet) the rows of the
// experiment directory after the run.
func logExperimentFiles(t *testing.T, root string) {
	t.Helper()
	type tally struct {
		files, bytes, rows int64
	}
	byName := map[string]*tally{}
	var total int64
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		name := d.Name()
		if strings.HasSuffix(name, ".json") && name != "report.json" {
			name = "*.json"
		}
		tl := byName[name]
		if tl == nil {
			tl = &tally{}
			byName[name] = tl
		}
		tl.files++
		tl.bytes += info.Size()
		total += info.Size()
		if strings.HasSuffix(name, ".parquet") {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			pf, err := parquet.OpenFile(f, info.Size())
			if err != nil {
				return err
			}
			tl.rows += pf.NumRows()
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		tl := byName[n]
		t.Logf("BENCH files %-22s count=%-6d bytes=%-12d rows=%d", n, tl.files, tl.bytes, tl.rows)
	}
	t.Logf("BENCH experiment dir %s total bytes=%d (%.1f MiB)", root, total, float64(total)/(1<<20))
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
