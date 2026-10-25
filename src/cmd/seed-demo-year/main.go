// Reset DATA_DIR and backfill one year of BTCUSDT (ma-cross) with VENUE_SIMULATE=1.
package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	signalrunner "github.com/MaxBernstetter/dagobert/src/calculation-signal/runner"
	tirunner "github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/runner"
	"github.com/MaxBernstetter/dagobert/src/internal/kpi"
	ingestrunner "github.com/MaxBernstetter/dagobert/src/ingest/runner"
	"github.com/MaxBernstetter/dagobert/src/shared"
)

const (
	symbol         = "BTCUSDT"
	threeHoursMs   = 3 * 60 * 60 * 1000
	paperVenueJSON = `{
  "coins": {
    "USDC": 100000,
    "USDT": 0,
    "BTC": 0,
    "ETH": 0
  }
}`
)

func main() {
	root, err := findRepoRoot()
	if err != nil {
		fatal(err)
	}
	dataDir := filepath.Join(root, "data")
	if v := os.Getenv("DATA_DIR"); v != "" {
		dataDir = v
	}

	fmt.Println("resetting", dataDir)
	if err := resetDataDir(dataDir); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "paper_venue.json"), []byte(paperVenueJSON), 0o644); err != nil {
		fatal(err)
	}

	applySeedEnv(dataDir)

	to := time.Now().UTC().UnixMilli()
	to -= to % (60 * 60 * 1000)
	from := to - 365*24*60*60*1000

	fmt.Printf("ingest %s …\n", symbol)
	code, detail := ingestrunner.Run(
		[]string{"-from", fmt.Sprint(from), "-to", fmt.Sprint(to), "-symbol", symbol},
		os.Getenv,
	)
	if err := failUnlessOK("ingest", code, detail); err != nil {
		fatal(err)
	}

	fmt.Println("technical indicators (backfill) …")
	detail, code = tirunner.Run(
		[]string{symbol, fmt.Sprint(from), fmt.Sprint(to)},
		dataDir,
		time.Now().UnixMilli(),
		false,
	)
	if err := failUnlessOK("calculation-technical-indicator", code, detail); err != nil {
		fatal(err)
	}

	closes, err := listCandleCloses(dataDir, symbol)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("pipeline: %d bars (fast replay: batched DB writes)\n", len(closes))
	minOpen := closes[0] - threeHoursMs
	maxOpen := closes[len(closes)-1] - threeHoursMs
	kpiWin, err := kpi.LoadWindow(dataDir, symbol, minOpen, maxOpen)
	if err != nil {
		fatal(err)
	}
	start := time.Now()
	if _, err := signalrunner.FastSimulatedPipeline(dataDir, kpiWin, "ma-cross", symbol, closes, nil); err != nil {
		fatal(err)
	}
	fmt.Printf("done in %s — open /strategy and /data\n", time.Since(start).Round(time.Second))
}

func applySeedEnv(dataDir string) {
	_ = os.Setenv("DATA_DIR", dataDir)
	_ = os.Setenv("VENUE_SIMULATE", "1")
	_ = os.Setenv("PLAYBOOK", "ma-cross.yaml")
	_ = os.Setenv("SOURCE", "backfill")
	if os.Getenv("BINANCE_BASE_URL") == "" {
		_ = os.Setenv("BINANCE_BASE_URL", "https://data-api.binance.vision")
	}
}

func jobOK(code int) bool {
	return code == 0 || code == 2
}

func failUnlessOK(stage string, code, detail int) error {
	if jobOK(code) {
		return nil
	}
	return fmt.Errorf("%s failed exit=%d detail=%d", stage, code, detail)
}

func listCandleCloses(dataDir, symbol string) ([]int64, error) {
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "technical_indicator.sqlite"))
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(
		`SELECT bar_open FROM kpi_revision WHERE symbol = ? AND algo = 'v1' ORDER BY bar_open`,
		symbol,
	)
	if err != nil {
		rows, err = db.Query(`SELECT bar_open FROM kpi WHERE symbol = ? ORDER BY bar_open`, symbol)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var barOpen int64
		if err := rows.Scan(&barOpen); err != nil {
			return nil, err
		}
		out = append(out, barOpen+threeHoursMs)
	}
	return out, rows.Err()
}

func resetDataDir(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	rmErr := os.RemoveAll(abs)
	if rmErr == nil {
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return err
		}
		return shared.EnsureDataLayout(abs)
	}
	if !isPermissionDenied(rmErr) {
		return rmErr
	}
	fmt.Println("clearing Docker-owned files under data/ …")
	if err := dockerClearDataDir(abs); err != nil {
		return fmt.Errorf(
			"%w\nhint: sudo chown -R \"$USER:$USER\" %s\nor: docker run --rm -v %s:/data alpine find /data -mindepth 1 -delete",
			err, abs, abs,
		)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return err
	}
	return shared.EnsureDataLayout(abs)
}

func isPermissionDenied(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrPermission) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "permission denied")
}

func dockerClearDataDir(abs string) error {
	cmd := exec.Command(
		"docker", "run", "--rm",
		"-v", abs+":/data",
		"alpine", "find", "/data", "-mindepth", "1", "-delete",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
