package experiment_test

import (
	"encoding/json"
	"testing"

	"github.com/MaxBernstetter/dagobert/src/backtest"
	"github.com/MaxBernstetter/dagobert/src/backtest/store"
)

// runMACross runs one ma-cross instance over the oscillating fixture and returns its workspace.
func runMACross(t *testing.T, cfg string) (string, error) {
	t.Helper()
	dataDir := tempDataDir(t)
	seedLiveMarker(t, dataDir)
	endMs := 240 * hour
	writeCrossingParquet(t, dataDir, symbol, endMs)

	cat, err := store.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cat.Close() })

	settings := store.DefaultExperimentSettings()
	settings.Base, settings.Quote = "BTC", "USDT"
	settings.FromMs, settings.ToMs = 0, endMs
	settings.RedownloadMissing = false
	rec, err := cat.CreateStudy("sizing", "", settings, []store.ExperimentInstanceSpec{
		{Label: "sized", Playbook: "ma-cross", StrategyConfig: json.RawMessage(cfg)},
	})
	if err != nil {
		return "", err
	}
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("VENUE_SIMULATE", "1")
	if err := backtest.RunExperiment(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}
	got, err := cat.GetExperiment(rec.ID)
	if err != nil || got.Status != store.StatusCompleted {
		t.Fatalf("status %q err %v", got.Status, err)
	}
	return store.ExperimentWorkspaceDir(dataDir, got.Instances[0]), nil
}

// buyQuotes lists the quote spent by each buy fill, oldest first.
func TestMACrossSizingBuysLessThanAllIn(t *testing.T) {
	legacyWS, _ := runMACross(t, `{"fastHr":3,"slowHr":6}`)
	sizedWS, _ := runMACross(t, `{"fastHr":3,"slowHr":6,"sizing":{}}`)
	legacy, sized := buyQuotes(t, legacyWS), buyQuotes(t, sizedWS)
	if len(legacy) == 0 || len(sized) == 0 {
		t.Fatalf("expected buys, legacy=%d sized=%d", len(legacy), len(sized))
	}
	if sized[0] >= legacy[0] {
		t.Fatalf("sized first buy %v should be below the legacy target weight %v", sized[0], legacy[0])
	}
	if countFills(t, sizedWS) < 2 {
		t.Fatal("expected the sized trade to exit as well")
	}
}

func TestMACrossPyramidAddsUnits(t *testing.T) {
	oneWS, _ := runMACross(t, `{"fastHr":3,"slowHr":6,"sizing":{"unitRiskPct":0.5}}`)
	twoWS, _ := runMACross(t, `{"fastHr":3,"slowHr":6,"sizing":{"unitRiskPct":0.5,"maxUnits":2}}`)
	if len(buyQuotes(t, twoWS)) < len(buyQuotes(t, oneWS)) {
		t.Fatal("maxUnits 2 must not buy less often than maxUnits 1")
	}
}

func TestMACrossInvalidSizingRejected(t *testing.T) {
	if _, err := runMACross(t, `{"fastHr":3,"slowHr":6,"sizing":{"unitRiskPct":3}}`); err == nil {
		t.Fatal("unitRiskPct 3 must be rejected")
	}
}
