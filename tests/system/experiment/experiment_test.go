package experiment_test

import (
	"database/sql"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MaxBernstetter/dagobert/src/backtest"
	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	"github.com/MaxBernstetter/dagobert/src/shared"
	"github.com/MaxBernstetter/dagobert/tests/system/internal/lakefile"
	_ "modernc.org/sqlite"
)

const (
	hour   = int64(3_600_000)
	symbol = "BTCUSDT"
)

type hourCandle = lakefile.Candle

func TestStudyMultiInstanceCorrectness(t *testing.T) {
	dataDir := tempDataDir(t)
	seedLiveMarker(t, dataDir)

	endMs := 240 * hour
	writeCrossingParquet(t, dataDir, symbol, endMs)

	url := pgScratchDB(t)

	settings := store.DefaultExperimentSettings()
	settings.Base = "BTC"
	settings.Quote = "USDT"
	settings.FromMs = 0
	settings.ToMs = endMs
	settings.RedownloadMissing = false

	cfgA, _ := json.Marshal(map[string]int{"fastHr": 3, "slowHr": 6})
	cfgB, _ := json.Marshal(map[string]int{"fastHr": 6, "slowHr": 12})
	rec, err := createExperimentPG(url, "multi-instance", settings, []fixtureSpec{
		{Label: "SMA 3/6", Playbook: "ma-cross", Config: cfgA},
		{Label: "SMA 6/12", Playbook: "ma-cross", Config: cfgB},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Instances) != 2 {
		t.Fatalf("instances %d", len(rec.Instances))
	}
	if rec.Instances[0].ConfigHash == rec.Instances[1].ConfigHash {
		t.Fatal("expected distinct config hashes")
	}
	if rec.Instances[0].PlaybookKey == rec.Instances[1].PlaybookKey {
		t.Fatal("expected distinct playbook keys")
	}

	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("DATABASE_URL", url)
	t.Setenv("VENUE_SIMULATE", "1")
	if err := backtest.RunExperiment(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}

	got := loadExperimentPG(t, url, rec.ID)
	if got.Status != store.StatusCompleted {
		t.Fatalf("status %q error %q", got.Status, got.Error)
	}
	for _, inst := range got.Instances {
		if inst.Status != store.StatusCompleted {
			t.Fatalf("instance %s status %q error %q", inst.Label, inst.Status, inst.Error)
		}
		if inst.StartedAt == nil || inst.FinishedAt == nil {
			t.Fatalf("instance %s missing timing started=%v finished=%v", inst.Label, inst.StartedAt, inst.FinishedAt)
		}
		if *inst.FinishedAt < *inst.StartedAt {
			t.Fatalf("instance %s finished_at %d < started_at %d", inst.Label, *inst.FinishedAt, *inst.StartedAt)
		}
		ws := instanceDir(dataDir, inst)
		reportPath := filepath.Join(ws, "report.json")
		body, err := os.ReadFile(reportPath)
		if err != nil {
			t.Fatalf("instance %s missing report.json: %v", inst.Label, err)
		}
		var report struct {
			Run struct {
				StartedAtMs  int64 `json:"startedAtMs"`
				FinishedAtMs int64 `json:"finishedAtMs"`
				DurationMs   int64 `json:"durationMs"`
			} `json:"run"`
			KPIs struct {
				Results []json.RawMessage `json:"results"`
			} `json:"kpis"`
			Statistics struct {
				MonthlyReturns []struct {
					Period string  `json:"period"`
					Return float64 `json:"return"`
				} `json:"monthlyReturns"`
			} `json:"statistics"`
		}
		if err := json.Unmarshal(body, &report); err != nil {
			t.Fatalf("instance %s report.json: %v", inst.Label, err)
		}
		if report.Run.StartedAtMs != *inst.StartedAt || report.Run.FinishedAtMs != *inst.FinishedAt {
			t.Fatalf("instance %s report timing mismatch columns vs file", inst.Label)
		}
		if report.Run.DurationMs != report.Run.FinishedAtMs-report.Run.StartedAtMs {
			t.Fatalf("instance %s durationMs %d != finished-started", inst.Label, report.Run.DurationMs)
		}
		if len(report.KPIs.Results) == 0 && len(report.Statistics.MonthlyReturns) == 0 {
			// Short fixture may have no fills; still require file + timing.
			t.Logf("instance %s report has no KPI results (possible on short range)", inst.Label)
		}
	}

	if _, err := os.Stat(filepath.Join(dataDir, "technical_indicator.sqlite")); !os.IsNotExist(err) {
		t.Fatal("prod technical_indicator.sqlite must not be created by experiment")
	}
	if liveRows(t, dataDir) != 1 {
		t.Fatal("live calculation_signal.sqlite marker row was overwritten")
	}

	wsA := instanceDir(dataDir, got.Instances[0])
	wsB := instanceDir(dataDir, got.Instances[1])
	buysA := countSignalsWithDecision(t, wsA, "buy")
	buysB := countSignalsWithDecision(t, wsB, "buy")
	fillsA := countFills(t, wsA)
	fillsB := countFills(t, wsB)
	closesA := decisionCloses(t, wsA, "buy")
	closesB := decisionCloses(t, wsB, "buy")
	t.Logf("3/6 buys=%d fills=%d closes=%v; 6/12 buys=%d fills=%d closes=%v", buysA, fillsA, closesA, buysB, fillsB, closesB)

	if buysA < 1 || fillsA < 1 {
		t.Fatalf("SMA 3/6 expected buy+fill on crossing fixture, buys=%d fills=%d", buysA, fillsA)
	}
	if buysB < 1 || fillsB < 1 {
		t.Fatalf("SMA 6/12 expected buy+fill on crossing fixture, buys=%d fills=%d", buysB, fillsB)
	}
	// Distinct configs must fire buys on different bars (golden divergence).
	if sameInt64Slice(closesA, closesB) {
		t.Fatalf("expected divergent buy candle closes for 3/6 vs 6/12; both %v", closesA)
	}

	keysA := distinctPlaybooks(t, wsA)
	keysB := distinctPlaybooks(t, wsB)
	if len(keysA) != 1 || keysA[0] != got.Instances[0].PlaybookKey {
		t.Fatalf("instance A playbook keys %v want [%s]", keysA, got.Instances[0].PlaybookKey)
	}
	if len(keysB) != 1 || keysB[0] != got.Instances[1].PlaybookKey {
		t.Fatalf("instance B playbook keys %v want [%s]", keysB, got.Instances[1].PlaybookKey)
	}

	endA := netCashFlow(t, wsA)
	endB := netCashFlow(t, wsB)
	if math.IsNaN(endA) || math.IsInf(endA, 0) || math.IsNaN(endB) || math.IsInf(endB, 0) {
		t.Fatal("net cash flow not finite")
	}
}

func TestStudySignalConsensusThresholds(t *testing.T) {
	dataDir := tempDataDir(t)
	seedLiveMarker(t, dataDir)

	endMs := 4000 * hour
	writeCrossingParquet(t, dataDir, symbol, endMs)

	url := pgScratchDB(t)

	settings := store.DefaultExperimentSettings()
	settings.Base = "BTC"
	settings.Quote = "USDT"
	settings.FromMs = 480 * hour
	settings.ToMs = endMs
	settings.RedownloadMissing = false

	cfgA, _ := json.Marshal(map[string]any{
		"rsiBull": 52, "rsiBear": 48,
		"volRatioBull": 1.1, "volRatioBear": 0.85,
		"atrStopMult":          2.5,
		"minBarsFlatAfterSell": 8, "minBarsLongBeforeSell": 4,
	})
	cfgB, _ := json.Marshal(map[string]any{
		"rsiBull": 60, "rsiBear": 40,
		"volRatioBull": 1.2, "volRatioBear": 0.8,
		"atrStopMult":          3.0,
		"minBarsFlatAfterSell": 4, "minBarsLongBeforeSell": 2,
	})
	rec, err := createExperimentPG(url, "consensus-thresholds", settings, []fixtureSpec{
		{Label: "default thresholds", Playbook: "signal-consensus", Config: cfgA},
		{Label: "loose thresholds", Playbook: "signal-consensus", Config: cfgB},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Instances) != 2 {
		t.Fatalf("instances %d", len(rec.Instances))
	}
	if rec.Instances[0].ConfigHash == rec.Instances[1].ConfigHash {
		t.Fatal("expected distinct config hashes")
	}
	if rec.Instances[0].PlaybookKey == rec.Instances[1].PlaybookKey {
		t.Fatal("expected distinct playbook keys")
	}

	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("DATABASE_URL", url)
	t.Setenv("VENUE_SIMULATE", "1")
	if err := backtest.RunExperiment(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}

	got := loadExperimentPG(t, url, rec.ID)
	if got.Status != store.StatusCompleted {
		t.Fatalf("status %q error %q", got.Status, got.Error)
	}
	for _, inst := range got.Instances {
		if inst.Status != store.StatusCompleted {
			t.Fatalf("instance %s status %q error %q", inst.Label, inst.Status, inst.Error)
		}
		if !strings.HasPrefix(inst.PlaybookKey, "signal-consensus.yaml#") {
			t.Fatalf("playbook key %q", inst.PlaybookKey)
		}
	}

	if liveRows(t, dataDir) != 1 {
		t.Fatal("live calculation_signal.sqlite marker row was overwritten")
	}
}

func tempDataDir(t *testing.T) string {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := shared.EnsureDataLayout(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

// writeCrossingParquet builds an oscillatory path so short vs long SMA crosses diverge.
func writeCrossingParquet(t *testing.T, dataDir, sym string, endMs int64) {
	t.Helper()
	hours := endMs / hour
	rows := make([]hourCandle, 0, hours)
	price := 100.0
	for i := 0; i < int(hours); i++ {
		// Alternating multi-hour trends with different wavelengths.
		phase := i % 36
		switch {
		case phase < 10:
			price -= 0.8
		case phase < 22:
			price += 1.1
		default:
			price -= 0.4
		}
		ot := int64(i) * hour
		rows = append(rows, hourCandle{
			OpenTime: ot, Open: price, High: price + 1, Low: price - 1,
			Close: price, Volume: 10, CloseTime: ot + hour - 1,
		})
	}
	writeRows(t, dataDir, rows)
}

// writeTrendingParquet rises with periodic sharp dips, so signal-consensus re-crosses
// SMA12 inside an established uptrend (momentum returns after each dip).
func writeTrendingParquet(t *testing.T, dataDir string, hours int) {
	t.Helper()
	rows := make([]hourCandle, 0, hours)
	price := 100.0
	for i := 0; i < hours; i++ {
		switch phase := i % 60; {
		case i < 100:
			price -= 0.2
		case phase >= 57:
			price -= 5
		case phase < 6:
			price += 14
		default:
			price += 2.5
		}
		ot := int64(i) * hour
		rows = append(rows, hourCandle{OpenTime: ot, Open: price, High: price + 1, Low: price - 1,
			Close: price, Volume: 10 + float64(i%7), CloseTime: ot + hour - 1})
	}
	writeRows(t, dataDir, rows)
}

func writeRows(t *testing.T, dataDir string, rows []hourCandle) {
	t.Helper()
	byYear := map[int][]hourCandle{}
	for _, r := range rows {
		y := shared.YearUTC(r.OpenTime)
		byYear[y] = append(byYear[y], r)
	}
	for y, part := range byYear {
		pqPath, err := shared.ParquetPath(dataDir, "binance", "BTC", "USDT", y)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(pqPath), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := lakefile.Write(pqPath, part); err != nil {
			t.Fatal(err)
		}
	}
}

func seedLiveMarker(t *testing.T, dataDir string) {
	t.Helper()
	dbPath := filepath.Join(dataDir, "calculation_signal.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`
CREATE TABLE signal (
  playbook TEXT NOT NULL,
  symbol TEXT NOT NULL,
  candle_close INTEGER NOT NULL,
  body TEXT NOT NULL,
  PRIMARY KEY (playbook, symbol, candle_close)
);
INSERT INTO signal (playbook, symbol, candle_close, body)
VALUES ('LIVE_MARKER', 'BTCUSDT', 1, '{}');
`)
	if err != nil {
		t.Fatal(err)
	}
}

func liveRows(t *testing.T, dataDir string) int {
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "calculation_signal.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM signal`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func sameInt64Slice(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestStudySignalConsensusSizing(t *testing.T) {
	dataDir := tempDataDir(t)
	seedLiveMarker(t, dataDir)
	endMs := 4000 * hour
	writeTrendingParquet(t, dataDir, int(endMs/hour))

	url := pgScratchDB(t)
	settings := store.DefaultExperimentSettings()
	settings.Base = "BTC"
	settings.Quote = "USDT"
	settings.FromMs = 480 * hour
	settings.ToMs = endMs
	settings.RedownloadMissing = false

	bad := []fixtureSpec{{Label: "bad", Playbook: "signal-consensus", Config: json.RawMessage(`{"sizing":{"unitRiskPct":5}}`)}}
	if _, err := createExperimentPG(url, "bad-sizing", settings, bad); err == nil {
		t.Fatal("invalid sizing must be rejected")
	}

	rec, err := createExperimentPG(url, "consensus-sizing", settings, []fixtureSpec{
		{Label: "legacy", Playbook: "signal-consensus"},
		{Label: "sized", Playbook: "signal-consensus", Config: json.RawMessage(`{"sizing":{"unitRiskPct":0.25}}`)},
		{Label: "below min", Playbook: "signal-consensus", Config: json.RawMessage(`{"sizing":{"maxWeightPct":0.0001}}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("DATABASE_URL", url)
	t.Setenv("VENUE_SIMULATE", "1")
	if err := backtest.RunExperiment(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}
	got := loadExperimentPG(t, url, rec.ID)
	for _, inst := range got.Instances {
		if inst.Status != store.StatusCompleted {
			t.Fatalf("instance %s status %q error %q", inst.Label, inst.Status, inst.Error)
		}
	}
	ws := map[string]string{}
	for _, inst := range got.Instances {
		ws[inst.Label] = instanceDir(dataDir, inst)
	}
	legacy, okL := firstBuyQuote(t, ws["legacy"])
	sized, okS := firstBuyQuote(t, ws["sized"])
	minQuote, okMin := firstBuyQuote(t, ws["below min"])
	t.Logf("first buy quote: legacy=%v sized=%v belowMin=%v (%v)", legacy, sized, minQuote, okMin)
	if !okL || !okS {
		t.Fatal("expected a buy fill for legacy and sized instances")
	}
	if sized == legacy || sized <= 0 {
		t.Fatalf("sized buy %v should differ from legacy target weight %v", sized, legacy)
	}
	if okMin {
		t.Fatal("buy below the venue minimum must be skipped")
	}
}
