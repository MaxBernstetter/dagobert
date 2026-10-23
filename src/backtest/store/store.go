package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	_ "modernc.org/sqlite"
)

const (
	StatusDraft     = "draft"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// Settings is the user-facing backtest configuration (snapshot at run time).
type Settings struct {
	Symbol             string  `json:"symbol"`
	Playbook           string  `json:"playbook"`
	FromMs             int64   `json:"fromMs"`
	ToMs               int64   `json:"toMs"`
	InitialUSDC        float64 `json:"initialUsdc"`
	InitialUSDT        float64 `json:"initialUsdt"`
	InitialBase        float64 `json:"initialBase"`
	TargetWeightPct    float64 `json:"targetWeightPct"`
	CashReserveQuote   float64 `json:"cashReserveQuote"`
	MinOrderQuote      float64 `json:"minOrderQuote"`
	RedownloadMissing  bool    `json:"redownloadMissing"`
	Algo               string  `json:"algo"`
	SpreadBps          float64 `json:"spreadBps"`
	SlippageBps        float64 `json:"slippageBps"`
	FeeBps             float64 `json:"feeBps"`
}

// DefaultSettings returns form defaults for a new backtest.
func DefaultSettings() Settings {
	now := time.Now().UTC()
	to := now.UnixMilli()
	to -= to % (60 * 60 * 1000)
	from := to - 365*24*60*60*1000
	return Settings{
		Symbol:            "BTCUSDT",
		Playbook:          "ma-cross",
		FromMs:            from,
		ToMs:              to,
		InitialUSDC:       100_000,
		InitialUSDT:       0,
		InitialBase:       0,
		TargetWeightPct:   95,
		CashReserveQuote:  100,
		MinOrderQuote:     1,
		RedownloadMissing: false,
		Algo:              "v1",
		SpreadBps:         4,
		SlippageBps:       2,
		FeeBps:            6,
	}
}

// Record is one catalog row.
type Record struct {
	ID               string
	Name             string
	Description      string
	Status           string
	Settings         Settings
	WorkspaceRelPath string
	CreatedAt        int64
	StartedAt        *int64
	FinishedAt       *int64
	Error            string
	ProgressDone      int
	ProgressTotal     int
	ProgressStage     int
	ProgressStages    int
	ProgressStageLabel string
}

// Catalog is data/backtests.sqlite.
type Catalog struct {
	path string
	db   *sql.DB
}

// Open opens or creates the catalog under dataDir.
func Open(dataDir string) (*Catalog, error) {
	if err := os.MkdirAll(filepath.Join(dataDir, "backtests"), 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dataDir, "backtests.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		db.Close()
		return nil, err
	}
	c := &Catalog{path: path, db: db}
	if err := c.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return c, nil
}

func (c *Catalog) Close() error {
	return c.db.Close()
}

func (c *Catalog) migrate() error {
	_, err := c.db.Exec(`
CREATE TABLE IF NOT EXISTS backtest (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  settings_json TEXT NOT NULL,
  workspace_rel_path TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  started_at INTEGER,
  finished_at INTEGER,
  error TEXT NOT NULL DEFAULT '',
  progress_bars_done INTEGER NOT NULL DEFAULT 0,
  progress_bars_total INTEGER NOT NULL DEFAULT 0,
  progress_stage INTEGER NOT NULL DEFAULT 0,
  progress_stages INTEGER NOT NULL DEFAULT 0,
  progress_stage_label TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS backtest_kpi (
  backtest_id TEXT NOT NULL,
  symbol TEXT NOT NULL,
  bar_open INTEGER NOT NULL,
  algo TEXT NOT NULL,
  body TEXT NOT NULL,
  origin TEXT NOT NULL,
  spec TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (backtest_id, symbol, bar_open, algo)
);
`)
	if err != nil {
		return err
	}
	return ensureColumns(c.db, "backtest", []struct{ name, ddl string }{
		{name: "progress_stage", ddl: "progress_stage INTEGER NOT NULL DEFAULT 0"},
		{name: "progress_stages", ddl: "progress_stages INTEGER NOT NULL DEFAULT 0"},
		{name: "progress_stage_label", ddl: "progress_stage_label TEXT NOT NULL DEFAULT ''"},
	})
}

// CreateDraft inserts a new draft backtest.
func (c *Catalog) CreateDraft(name, description string, settings Settings) (Record, error) {
	if strings.TrimSpace(name) == "" {
		return Record{}, errors.New("name required")
	}
	if settings.Symbol == "" {
		settings = DefaultSettings()
	}
	if settings.Algo == "" {
		settings.Algo = "v1"
	}
	id := uuid.NewString()
	rel := filepath.Join("backtests", id)
	created := time.Now().UnixMilli()
	body, err := json.Marshal(settings)
	if err != nil {
		return Record{}, err
	}
	_, err = c.db.Exec(
		`INSERT INTO backtest (id, name, description, status, settings_json, workspace_rel_path, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, name, description, StatusDraft, string(body), rel, created,
	)
	if err != nil {
		return Record{}, err
	}
	if err := os.MkdirAll(filepath.Join(filepath.Dir(c.path), rel), 0o755); err != nil {
		return Record{}, err
	}
	return Record{
		ID:               id,
		Name:             name,
		Description:      description,
		Status:           StatusDraft,
		Settings:         settings,
		WorkspaceRelPath: rel,
		CreatedAt:        created,
	}, nil
}

// List returns all backtests newest first.
func (c *Catalog) List() ([]Record, error) {
	rows, err := c.db.Query(
		`SELECT id, name, description, status, settings_json, workspace_rel_path, created_at,
		        started_at, finished_at, error, progress_bars_done, progress_bars_total,
		        progress_stage, progress_stages, progress_stage_label
		 FROM backtest ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// Get loads one backtest by id.
func (c *Catalog) Get(id string) (Record, error) {
	row := c.db.QueryRow(
		`SELECT id, name, description, status, settings_json, workspace_rel_path, created_at,
		        started_at, finished_at, error, progress_bars_done, progress_bars_total,
		        progress_stage, progress_stages, progress_stage_label
		 FROM backtest WHERE id = ?`, id,
	)
	return scanRecordRow(row)
}

func scanRecord(rows *sql.Rows) (Record, error) {
	var rec Record
	var settingsJSON string
	var started, finished sql.NullInt64
	if err := rows.Scan(
		&rec.ID, &rec.Name, &rec.Description, &rec.Status, &settingsJSON, &rec.WorkspaceRelPath,
		&rec.CreatedAt, &started, &finished, &rec.Error, &rec.ProgressDone, &rec.ProgressTotal,
		&rec.ProgressStage, &rec.ProgressStages, &rec.ProgressStageLabel,
	); err != nil {
		return Record{}, err
	}
	return finishScanRecord(&rec, settingsJSON, started, finished)
}

func scanRecordRow(row *sql.Row) (Record, error) {
	var rec Record
	var settingsJSON string
	var started, finished sql.NullInt64
	err := row.Scan(
		&rec.ID, &rec.Name, &rec.Description, &rec.Status, &settingsJSON, &rec.WorkspaceRelPath,
		&rec.CreatedAt, &started, &finished, &rec.Error, &rec.ProgressDone, &rec.ProgressTotal,
		&rec.ProgressStage, &rec.ProgressStages, &rec.ProgressStageLabel,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, fmt.Errorf("backtest not found")
	}
	if err != nil {
		return Record{}, err
	}
	return finishScanRecord(&rec, settingsJSON, started, finished)
}

func finishScanRecord(rec *Record, settingsJSON string, started, finished sql.NullInt64) (Record, error) {
	if started.Valid {
		v := started.Int64
		rec.StartedAt = &v
	}
	if finished.Valid {
		v := finished.Int64
		rec.FinishedAt = &v
	}
	if err := json.Unmarshal([]byte(settingsJSON), &rec.Settings); err != nil {
		return Record{}, err
	}
	return *rec, nil
}

// UpdateDraft updates name, description, and settings for draft or completed backtests.
func (c *Catalog) UpdateDraft(id string, name, description string, settings Settings) error {
	rec, err := c.Get(id)
	if err != nil {
		return err
	}
	if rec.Status == StatusRunning {
		return errors.New("cannot edit while running")
	}
	body, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = c.db.Exec(
		`UPDATE backtest SET name = ?, description = ?, settings_json = ?, status = ? WHERE id = ?`,
		name, description, string(body), StatusDraft, id,
	)
	return err
}

// MarkRunning sets status and resets staged progress.
func (c *Catalog) MarkRunning(id string, _ int) error {
	now := time.Now().UnixMilli()
	_, err := c.db.Exec(
		`UPDATE backtest SET status = ?, started_at = ?, finished_at = NULL, error = '',
		 progress_bars_done = 0, progress_bars_total = 0,
		 progress_stage = 1, progress_stages = ?, progress_stage_label = ? WHERE id = ?`,
		StatusRunning, now, BacktestStageTotal, StageLabelPrepare, id,
	)
	return err
}

// MarkCompleted sets status completed.
func (c *Catalog) MarkCompleted(id string) error {
	now := time.Now().UnixMilli()
	_, err := c.db.Exec(
		`UPDATE backtest SET status = ?, finished_at = ?, error = '' WHERE id = ?`,
		StatusCompleted, now, id,
	)
	return err
}

// MarkFailed records failure.
func (c *Catalog) MarkFailed(id string, message string) error {
	now := time.Now().UnixMilli()
	_, err := c.db.Exec(
		`UPDATE backtest SET status = ?, finished_at = ?, error = ? WHERE id = ?`,
		StatusFailed, now, message, id,
	)
	return err
}

// WorkspaceDir resolves the absolute workspace for a record.
func WorkspaceDir(dataDir string, rec Record) string {
	return filepath.Join(dataDir, rec.WorkspaceRelPath)
}
