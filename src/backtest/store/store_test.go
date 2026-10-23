package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MaxBernstetter/trading-copilot/src/backtest/store"
)

func TestCatalogCreateList(t *testing.T) {
	dir := t.TempDir()
	cat, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer cat.Close()

	rec, err := cat.CreateDraft("test run", "notes", store.DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != store.StatusDraft {
		t.Fatalf("status %s", rec.Status)
	}
	list, err := cat.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v err %v", list, err)
	}
	ws := store.WorkspaceDir(dir, rec)
	if _, err := os.Stat(ws); err != nil {
		t.Fatalf("workspace: %v", err)
	}
	if !filepath.IsAbs(ws) {
		t.Fatal("expected abs workspace")
	}
}

func TestSetRunProgress(t *testing.T) {
	dir := t.TempDir()
	cat, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer cat.Close()

	rec, err := cat.CreateDraft("progress", "", store.DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	if err := cat.MarkRunning(rec.ID, 0); err != nil {
		t.Fatal(err)
	}
	if err := cat.SetRunProgress(rec.ID, 2, store.StageLabelMarketData, 0, 1); err != nil {
		t.Fatal(err)
	}
	got, err := cat.Get(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProgressStage != 2 || got.ProgressStages != store.BacktestStageTotal {
		t.Fatalf("stage %+v", got)
	}
	if got.ProgressStageLabel != store.StageLabelMarketData {
		t.Fatalf("label %q", got.ProgressStageLabel)
	}
}
