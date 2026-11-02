package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "modernc.org/sqlite"
)

const (
	threeHours = 3 * time.Hour
	hourMS     = int64(time.Hour / time.Millisecond)
)

// jobMu serializes data job containers (backfill runs one at a time).
var jobMu sync.Mutex

func main() {
	os.Exit(serve())
}

// serve prepares the data dir and serves the HTTP API (backfill, connectors, Experiment API).
func serve() int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	clearStaleBackfill()
	dataDir, err := hostDataDirForJobs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "orchestrator: data dir: %s\n", err)
		return 1
	}
	if err := ensureJobDataLayout(dataDir); err != nil {
		fmt.Fprintf(os.Stderr, "orchestrator: data layout: %s\n", err)
		return 1
	}
	serveBackfill(ctx)
	return 0
}

type taskInput struct {
	Stage       string `json:"stage"`
	CandleClose int64  `json:"candleClose"`
	Symbol      string `json:"symbol"`
	Provider    string `json:"provider,omitempty"`
	From        int64  `json:"from,omitempty"`
}

// runJobLocked runs one data job container (ingest or indicators). Callers hold jobMu.
func runJobLocked(ctx context.Context, in taskInput) (int, error) {
	if in.CandleClose <= 0 || in.Symbol == "" {
		return 1, fmt.Errorf("missing candle or symbol")
	}
	barOpen := in.CandleClose - int64(threeHours/time.Millisecond)
	data, err := hostDataDirForJobs()
	if err != nil {
		return 1, err
	}
	if err := ensureJobDataLayout(data); err != nil {
		return 1, err
	}
	args := []string{"run", "--rm"}
	args = append(args, dockerJobUserArgs()...)
	args = append(args, "-e", "DATA_DIR=/data", "-v", data+":/data")
	args = append(args, dockerDataPlaneEnvArgs()...)
	switch in.Stage {
	case "ingest":
		from := in.From
		provider := strings.TrimSpace(in.Provider)
		if provider == "" {
			provider = "binance"
		}
		base, quote := splitSeries(in.Symbol)
		if from <= 0 {
			from = ingestFrom(provider, base, quote, barOpen)
		}
		args = append(args,
			"-e", "BINANCE_BASE_URL="+env("BINANCE_BASE_URL", "https://data-api.binance.vision"),
			"dagobert-ingest",
			"-from", strconv.FormatInt(from, 10),
			"-to", strconv.FormatInt(in.CandleClose, 10),
			"-base", base,
			"-quote", quote,
			"-provider", provider,
		)
	case "indicators":
		args = append(args,
			"-e", "SOURCE=backfill",
			"dagobert-calculation-technical-indicator",
			in.Symbol,
			strconv.FormatInt(in.From, 10),
			strconv.FormatInt(in.CandleClose, 10),
		)
	default:
		return 1, fmt.Errorf("unknown stage %s", in.Stage)
	}
	fmt.Printf("orchestrator: %s %s %d\n", in.Stage, in.Symbol, in.CandleClose)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	code := 0
	if err == nil {
		return 0, nil
	}
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
		return code, nil
	}
	return 1, err
}

// ingestFrom starts at the hour after the newest stored open when that hour
// is older than the current bar, so one ingest fills the gap and the current bar.
func splitSeries(sym string) (base, quote string) {
	sym = strings.ToUpper(strings.TrimSpace(sym))
	quotes := []string{"USDT", "USDC", "BUSD", "FDUSD", "TUSD", "BTC", "ETH", "BNB", "EUR", "TRY", "BRL", "GBP", "AUD", "USD"}
	for _, q := range quotes {
		if strings.HasSuffix(sym, q) && len(sym) > len(q) {
			return sym[:len(sym)-len(q)], q
		}
	}
	if sym == "" {
		return "BTC", "USDT"
	}
	return sym, "USDT"
}

func ingestFrom(provider, base, quote string, barOpen int64) int64 {
	latest, ok := latestStored(provider, base, quote)
	if !ok {
		return barOpen
	}
	next := latest + hourMS
	if next < barOpen {
		fmt.Printf("orchestrator: %s %s/%s behind, ingest from %d through the current bar\n", provider, base, quote, next)
		return next
	}
	return barOpen
}

func latestStored(provider, base, quote string) (int64, bool) {
	path := filepath.Join(env("LIVE_DIR", "/data"), "ingest.sqlite")
	if _, err := os.Stat(path); err != nil {
		return 0, false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return 0, false
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs), RawQuery: "_pragma=busy_timeout(5000)"}
	db, err := sql.Open("sqlite", u.String()+"&mode=ro")
	if err != nil {
		return 0, false
	}
	defer db.Close()
	var latest sql.NullInt64
	err = db.QueryRow(
		`SELECT MAX(open_time) FROM stored WHERE provider = ? AND base = ? AND quote = ?`,
		provider, base, quote,
	).Scan(&latest)
	if err != nil || !latest.Valid {
		return 0, false
	}
	return latest.Int64, true
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
