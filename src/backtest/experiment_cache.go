package backtest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	tirunner "github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/runner"
	"github.com/MaxBernstetter/dagobert/src/internal/kpi"
	"github.com/MaxBernstetter/dagobert/src/internal/signalstore"
)

// ExperimentCacheDir is experiments/{id}/.cache under DATA_DIR.
func ExperimentCacheDir(dataDir, experimentID string) string {
	return filepath.Join(dataDir, "experiments", experimentID, ".cache")
}

type experimentPrepareMeta struct {
	ExperimentID       string                `json:"experimentId"`
	From          int64                 `json:"fromMs"`
	To            int64                 `json:"toMs"`
	NowMS         int64                 `json:"nowMs"`
	FailedSymbols map[string]string     `json:"failedSymbols"`
	Settings      store.ExperimentSettings   `json:"settings"`
	InstanceIDs   []string              `json:"instanceIds"`
	InstTotal     int                   `json:"instTotal"`
}

type experimentSymKPI struct {
	win    *kpi.Window
	closes []int64
	rows   []tirunner.KPIRow
}

type cachedKPIFile struct {
	Symbol string          `json:"symbol"`
	Closes []int64         `json:"closes"`
	Rows   []cachedKPIRow  `json:"rows"`
}

type cachedKPIRow struct {
	Symbol  string `json:"symbol"`
	BarOpen int64  `json:"barOpen"`
	Body    string `json:"body"`
}

func WriteExperimentPrepareMeta(dataDir, experimentID string, meta experimentPrepareMeta) error {
	dir := ExperimentCacheDir(dataDir, experimentID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "prepare_meta.json"), raw, 0o644); err != nil {
		return err
	}
	instRaw, err := json.MarshalIndent(map[string][]string{"instanceIds": meta.InstanceIDs}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "instances.json"), instRaw, 0o644)
}

func ReadExperimentPrepareMeta(dataDir, experimentID string) (experimentPrepareMeta, error) {
	path := filepath.Join(ExperimentCacheDir(dataDir, experimentID), "prepare_meta.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return experimentPrepareMeta{}, fmt.Errorf("read prepare cache: %w", err)
	}
	var meta experimentPrepareMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return experimentPrepareMeta{}, err
	}
	return meta, nil
}

func ReadExperimentInstanceList(dataDir, experimentID string) ([]string, error) {
	path := filepath.Join(ExperimentCacheDir(dataDir, experimentID), "instances.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read instances cache: %w", err)
	}
	var doc struct {
		InstanceIDs []string `json:"instanceIds"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	return doc.InstanceIDs, nil
}

func WriteExperimentSymKPI(dataDir, experimentID, symbol string, bundle experimentSymKPI) error {
	dir := filepath.Join(ExperimentCacheDir(dataDir, experimentID), "kpi")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	rows := make([]cachedKPIRow, len(bundle.rows))
	for i, r := range bundle.rows {
		rows[i] = cachedKPIRow{Symbol: r.Symbol, BarOpen: r.BarOpen, Body: r.Body}
	}
	file := cachedKPIFile{Symbol: symbol, Closes: bundle.closes, Rows: rows}
	raw, err := json.Marshal(file)
	if err != nil {
		return err
	}
	safe := strings.ReplaceAll(symbol, "/", "_")
	return os.WriteFile(filepath.Join(dir, safe+".json"), raw, 0o644)
}

func LoadExperimentSymKPI(dataDir, experimentID string) (map[string]experimentSymKPI, error) {
	dir := filepath.Join(ExperimentCacheDir(dataDir, experimentID), "kpi")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read kpi cache dir: %w", err)
	}
	aggMS := signalstore.AggregationDurationMS()
	out := make(map[string]experimentSymKPI)
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, ent.Name()))
		if err != nil {
			return nil, err
		}
		var file cachedKPIFile
		if err := json.Unmarshal(raw, &file); err != nil {
			return nil, err
		}
		sym := file.Symbol
		if sym == "" {
			continue
		}
		rows := make([]tirunner.KPIRow, len(file.Rows))
		bodies := make(map[int64]string, len(file.Rows))
		for i, r := range file.Rows {
			rows[i] = tirunner.KPIRow{Symbol: sym, BarOpen: r.BarOpen, Body: r.Body}
			bodies[r.BarOpen] = r.Body
		}
		win, err := kpi.LoadWindowBodies(bodies)
		if err != nil {
			return nil, fmt.Errorf("kpi window %s: %w", sym, err)
		}
		closes := file.Closes
		if len(closes) == 0 {
			for _, row := range rows {
				closeMS := row.BarOpen + aggMS
				closes = append(closes, closeMS)
			}
		}
		out[sym] = experimentSymKPI{win: win, closes: closes, rows: rows}
	}
	return out, nil
}

func WriteAllExperimentSymKPI(dataDir, experimentID string, bySymbol map[string]experimentSymKPI) error {
	for sym, bundle := range bySymbol {
		if err := WriteExperimentSymKPI(dataDir, experimentID, sym, bundle); err != nil {
			return err
		}
	}
	return nil
}
