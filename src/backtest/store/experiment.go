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

	macross "github.com/MaxBernstetter/dagobert/src/calculation-signal/playbooks/ma-cross"
	signalconsensus "github.com/MaxBernstetter/dagobert/src/calculation-signal/playbooks/signal-consensus"
	signalrunner "github.com/MaxBernstetter/dagobert/src/calculation-signal/runner"
	"github.com/MaxBernstetter/dagobert/src/internal/signalstore"
)

// ExperimentSettings is shared experiment configuration (provider/base/quote/range/capital/fees).
type ExperimentSettings struct {
	Provider          string  `json:"provider"`
	Base              string  `json:"base"`
	Quote             string  `json:"quote"`
	FromMs            int64   `json:"fromMs"`
	ToMs              int64   `json:"toMs"`
	InitialUSDC       float64 `json:"initialUsdc"`
	InitialUSDT       float64 `json:"initialUsdt"`
	InitialBase       float64 `json:"initialBase"`
	TargetWeightPct   float64 `json:"targetWeightPct"`
	CashReserveQuote  float64 `json:"cashReserveQuote"`
	MinOrderQuote     float64 `json:"minOrderQuote"`
	RedownloadMissing bool    `json:"redownloadMissing"`
	Algo              string  `json:"algo"`
	SpreadBps         float64 `json:"spreadBps"`
	SlippageBps       float64 `json:"slippageBps"`
	FeeBps            float64 `json:"feeBps"`
}

// PipelineSymbol is the KPI/signal/execution series id for this experiment market.
func (s ExperimentSettings) PipelineSymbol() string {
	return pipelineSymbol(s.Provider, s.Base, s.Quote)
}

// DefaultExperimentSettings returns form defaults for a new experiment.
func DefaultExperimentSettings() ExperimentSettings {
	s := DefaultSettings()
	return ExperimentSettings{
		Provider:          s.Provider,
		Base:              s.Base,
		Quote:             s.Quote,
		FromMs:            s.FromMs,
		ToMs:              s.ToMs,
		InitialUSDC:       s.InitialUSDC,
		InitialUSDT:       s.InitialUSDT,
		InitialBase:       s.InitialBase,
		TargetWeightPct:   s.TargetWeightPct,
		CashReserveQuote:  s.CashReserveQuote,
		MinOrderQuote:     s.MinOrderQuote,
		RedownloadMissing: s.RedownloadMissing,
		Algo:              s.Algo,
		SpreadBps:         s.SpreadBps,
		SlippageBps:       s.SlippageBps,
		FeeBps:            s.FeeBps,
	}
}

// ExperimentInstanceSpec is one strategy config variant at create time.
type ExperimentInstanceSpec struct {
	Label          string          `json:"label"`
	Playbook       string          `json:"playbook"`
	Symbol         string          `json:"symbol,omitempty"`
	StrategyConfig json.RawMessage `json:"strategyConfig"`
}

// ExperimentInstance is one catalogued strategy variant under a experiment.
type ExperimentInstance struct {
	ID               string
	ExperimentID     string
	Label            string
	Playbook         string
	Symbol           string // empty → fall back to experiment settings.symbol
	StrategyConfig   json.RawMessage
	ConfigHash       string
	PlaybookKey      string
	WorkspaceRelPath string
	Status           string
	Error            string
	StartedAt        *int64          // instance sim wall start (ms), nil if not run
	FinishedAt       *int64          // instance sim wall end (ms)
	RunReportJSON    json.RawMessage // full RunReport JSON; empty if unset
}

// InstanceRunTiming is optional wall-clock written on complete/fail.
// Full report lives at experiments/.../instances/.../report.json (not SQLite).
type InstanceRunTiming struct {
	StartedAtMs  int64
	FinishedAtMs int64
}

// ExperimentRecord is one experiment with its instances.
type ExperimentRecord struct {
	ID                    string
	Name                  string
	Description           string
	Status                string
	Settings              ExperimentSettings
	CreatedAt             int64
	StartedAt             *int64
	FinishedAt            *int64
	Error                 string
	ProgressStage         int
	ProgressStages        int
	ProgressStageLabel    string
	ProgressInstanceDone  int
	ProgressInstanceTotal int
	Instances             []ExperimentInstance
}

const ExperimentStageTotal = 4

const (
	ExperimentStageLabelPrepare    = "Preparing"
	ExperimentStageLabelMarketData = "Market data"
	ExperimentStageLabelIndicators = "Indicators"
	ExperimentStageLabelSimulation = "Instance simulation"
)

func migrateExperimentTables(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS experiment (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  settings_json TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  started_at INTEGER,
  finished_at INTEGER,
  error TEXT NOT NULL DEFAULT '',
  progress_stage INTEGER NOT NULL DEFAULT 0,
  progress_stages INTEGER NOT NULL DEFAULT 0,
  progress_stage_label TEXT NOT NULL DEFAULT '',
  progress_instance_done INTEGER NOT NULL DEFAULT 0,
  progress_instance_total INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS experiment_instance (
  id TEXT PRIMARY KEY,
  experiment_id TEXT NOT NULL,
  label TEXT NOT NULL DEFAULT '',
  playbook TEXT NOT NULL,
  strategy_config_json TEXT NOT NULL,
  config_hash TEXT NOT NULL,
  playbook_key TEXT NOT NULL,
  workspace_rel_path TEXT NOT NULL,
  status TEXT NOT NULL,
  error TEXT NOT NULL DEFAULT '',
  FOREIGN KEY (experiment_id) REFERENCES experiment(id)
);
CREATE TABLE IF NOT EXISTS experiment_kpi (
  experiment_id TEXT NOT NULL,
  symbol TEXT NOT NULL,
  bar_open INTEGER NOT NULL,
  algo TEXT NOT NULL,
  body TEXT NOT NULL,
  origin TEXT NOT NULL,
  spec TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (experiment_id, symbol, bar_open, algo)
);
CREATE INDEX IF NOT EXISTS idx_experiment_instance_experiment_id ON experiment_instance(experiment_id);
`)
	if err != nil {
		return err
	}
	if _, err := db.Exec(`ALTER TABLE experiment_instance ADD COLUMN symbol TEXT NOT NULL DEFAULT ''`); err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return err
	}
	for _, ddl := range []string{
		`ALTER TABLE experiment_instance ADD COLUMN started_at INTEGER`,
		`ALTER TABLE experiment_instance ADD COLUMN finished_at INTEGER`,
		`ALTER TABLE experiment_instance ADD COLUMN run_report_json TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE experiment ADD COLUMN grid_json TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := db.Exec(ddl); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return err
		}
	}
	return migrateInstanceMetricColumns(db)
}

// CreateStudy inserts a draft experiment with instances (configurable playbook configs validated).
func (c *Catalog) CreateStudy(name, description string, settings ExperimentSettings, specs []ExperimentInstanceSpec) (ExperimentRecord, error) {
	if strings.TrimSpace(name) == "" {
		return ExperimentRecord{}, errors.New("name required")
	}
	if len(specs) == 0 {
		return ExperimentRecord{}, errors.New("at least one instance required")
	}
	if settings.Base == "" || settings.Quote == "" {
		settings = DefaultExperimentSettings()
	}
	if settings.Algo == "" {
		settings.Algo = "v1"
	}
	if strings.TrimSpace(settings.Provider) == "" {
		settings.Provider = "binance"
	}
	settings.Base = strings.ToUpper(strings.TrimSpace(settings.Base))
	settings.Quote = strings.ToUpper(strings.TrimSpace(settings.Quote))

	aggMS := signalstore.AggregationDurationMS()
	experimentID := uuid.NewString()
	created := time.Now().UnixMilli()
	settingsBody, err := json.Marshal(settings)
	if err != nil {
		return ExperimentRecord{}, err
	}

	tx, err := c.db.Begin()
	if err != nil {
		return ExperimentRecord{}, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO experiment (id, name, description, status, settings_json, created_at,
		 progress_instance_total) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		experimentID, name, description, StatusDraft, string(settingsBody), created, len(specs),
	)
	if err != nil {
		return ExperimentRecord{}, err
	}

	instances := make([]ExperimentInstance, 0, len(specs))
	seenHash := map[string]struct{}{}
	dataRoot := filepath.Dir(c.path)

	for i, spec := range specs {
		playbook := strings.TrimSpace(spec.Playbook)
		if playbook == "" {
			playbook = "ma-cross"
		}
		ref, err := signalrunner.ResolvePlaybookRef(playbook)
		if err != nil {
			return ExperimentRecord{}, err
		}
		configJSON, hash, defaultLabel, err := normalizeExperimentInstanceConfig(ref.Slug, spec.StrategyConfig, aggMS)
		if err != nil {
			return ExperimentRecord{}, err
		}
		sym := strings.TrimSpace(spec.Symbol)
		if sym == "" {
			sym = settings.PipelineSymbol()
		}
		if sym == "" {
			return ExperimentRecord{}, fmt.Errorf("instance %d: missing market", i+1)
		}
		uniq := sym + "|" + hash
		if _, dup := seenHash[uniq]; dup {
			return ExperimentRecord{}, fmt.Errorf("duplicate strategy config for %s (hash %s)", sym, hash[:8])
		}
		seenHash[uniq] = struct{}{}

		instID := uuid.NewString()
		label := strings.TrimSpace(spec.Label)
		if label == "" {
			label = sym + " " + defaultLabel
		}
		playbookKey := signalrunner.InstancePlaybookKey(ref.ID, hash)
		rel := filepath.Join("experiments", experimentID, "instances", instID)
		if err := os.MkdirAll(filepath.Join(dataRoot, rel), 0o755); err != nil {
			return ExperimentRecord{}, err
		}
		_, err = tx.Exec(
			`INSERT INTO experiment_instance
			 (id, experiment_id, label, playbook, symbol, strategy_config_json, config_hash, playbook_key, workspace_rel_path, status)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			instID, experimentID, label, ref.Slug, sym, string(configJSON), hash, playbookKey, rel, StatusDraft,
		)
		if err != nil {
			return ExperimentRecord{}, err
		}
		instances = append(instances, ExperimentInstance{
			ID:               instID,
			ExperimentID:     experimentID,
			Label:            label,
			Playbook:         ref.Slug,
			Symbol:           sym,
			StrategyConfig:   configJSON,
			ConfigHash:       hash,
			PlaybookKey:      playbookKey,
			WorkspaceRelPath: rel,
			Status:           StatusDraft,
		})
	}

	if err := tx.Commit(); err != nil {
		return ExperimentRecord{}, err
	}

	rec := ExperimentRecord{
		ID:                    experimentID,
		Name:                  name,
		Description:           description,
		Status:                StatusDraft,
		Settings:              settings,
		CreatedAt:             created,
		ProgressInstanceTotal: len(instances),
		Instances:             instances,
	}
	return rec, nil
}

// ListExperiments returns experiments newest first (with instances).
func (c *Catalog) ListExperiments() ([]ExperimentRecord, error) {
	rows, err := c.db.Query(
		`SELECT id, name, description, status, settings_json, created_at, started_at, finished_at, error,
		        progress_stage, progress_stages, progress_stage_label,
		        progress_instance_done, progress_instance_total
		 FROM experiment ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExperimentRecord
	for rows.Next() {
		rec, err := scanStudyRow(rows)
		if err != nil {
			return nil, err
		}
		insts, err := c.listInstances(rec.ID)
		if err != nil {
			return nil, err
		}
		rec.Instances = insts
		out = append(out, rec)
	}
	return out, rows.Err()
}

// GetExperiment loads one experiment with instances.
func (c *Catalog) GetExperiment(id string) (ExperimentRecord, error) {
	return c.getExperimentSQLite(id)
}

func (c *Catalog) getExperimentSQLite(id string) (ExperimentRecord, error) {
	row := c.db.QueryRow(
		`SELECT id, name, description, status, settings_json, created_at, started_at, finished_at, error,
		        progress_stage, progress_stages, progress_stage_label,
		        progress_instance_done, progress_instance_total
		 FROM experiment WHERE id = ?`, id,
	)
	rec, err := scanStudyQueryRow(row)
	if err != nil {
		return ExperimentRecord{}, err
	}
	insts, err := c.listInstances(id)
	if err != nil {
		return ExperimentRecord{}, err
	}
	rec.Instances = insts
	return rec, nil
}

// GetExperimentInstance loads one instance by id.
func (c *Catalog) GetExperimentInstance(instanceID string) (ExperimentInstance, error) {
	return c.getExperimentInstanceSQLite(instanceID)
}

func (c *Catalog) getExperimentInstanceSQLite(instanceID string) (ExperimentInstance, error) {
	row := c.db.QueryRow(
		`SELECT id, experiment_id, label, playbook, COALESCE(symbol,''), strategy_config_json, config_hash, playbook_key,
		        workspace_rel_path, status, error, started_at, finished_at, COALESCE(run_report_json,'')
		 FROM experiment_instance WHERE id = ?`, instanceID,
	)
	return scanInstanceRow(row)
}

func (c *Catalog) listInstances(experimentID string) ([]ExperimentInstance, error) {
	rows, err := c.db.Query(
		`SELECT id, experiment_id, label, playbook, COALESCE(symbol,''), strategy_config_json, config_hash, playbook_key,
		        workspace_rel_path, status, error, started_at, finished_at, COALESCE(run_report_json,'')
		 FROM experiment_instance WHERE experiment_id = ? ORDER BY label, id`, experimentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExperimentInstance
	for rows.Next() {
		inst, err := scanInstance(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inst)
	}
	return out, rows.Err()
}

func scanStudyRow(rows *sql.Rows) (ExperimentRecord, error) {
	var rec ExperimentRecord
	var settingsJSON string
	var started, finished sql.NullInt64
	if err := rows.Scan(
		&rec.ID, &rec.Name, &rec.Description, &rec.Status, &settingsJSON, &rec.CreatedAt,
		&started, &finished, &rec.Error, &rec.ProgressStage, &rec.ProgressStages, &rec.ProgressStageLabel,
		&rec.ProgressInstanceDone, &rec.ProgressInstanceTotal,
	); err != nil {
		return ExperimentRecord{}, err
	}
	return finishStudyScan(&rec, settingsJSON, started, finished)
}

func scanStudyQueryRow(row *sql.Row) (ExperimentRecord, error) {
	var rec ExperimentRecord
	var settingsJSON string
	var started, finished sql.NullInt64
	err := row.Scan(
		&rec.ID, &rec.Name, &rec.Description, &rec.Status, &settingsJSON, &rec.CreatedAt,
		&started, &finished, &rec.Error, &rec.ProgressStage, &rec.ProgressStages, &rec.ProgressStageLabel,
		&rec.ProgressInstanceDone, &rec.ProgressInstanceTotal,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ExperimentRecord{}, fmt.Errorf("experiment not found")
	}
	if err != nil {
		return ExperimentRecord{}, err
	}
	return finishStudyScan(&rec, settingsJSON, started, finished)
}

func finishStudyScan(rec *ExperimentRecord, settingsJSON string, started, finished sql.NullInt64) (ExperimentRecord, error) {
	if started.Valid {
		v := started.Int64
		rec.StartedAt = &v
	}
	if finished.Valid {
		v := finished.Int64
		rec.FinishedAt = &v
	}
	if err := json.Unmarshal([]byte(settingsJSON), &rec.Settings); err != nil {
		return ExperimentRecord{}, err
	}
	return *rec, nil
}

func scanInstance(rows *sql.Rows) (ExperimentInstance, error) {
	var inst ExperimentInstance
	var cfg, reportJSON string
	var started, finished sql.NullInt64
	if err := rows.Scan(
		&inst.ID, &inst.ExperimentID, &inst.Label, &inst.Playbook, &inst.Symbol, &cfg, &inst.ConfigHash,
		&inst.PlaybookKey, &inst.WorkspaceRelPath, &inst.Status, &inst.Error,
		&started, &finished, &reportJSON,
	); err != nil {
		return ExperimentInstance{}, err
	}
	inst.StrategyConfig = json.RawMessage(cfg)
	finishInstanceScan(&inst, started, finished, reportJSON)
	return inst, nil
}

func scanInstanceRow(row *sql.Row) (ExperimentInstance, error) {
	var inst ExperimentInstance
	var cfg, reportJSON string
	var started, finished sql.NullInt64
	err := row.Scan(
		&inst.ID, &inst.ExperimentID, &inst.Label, &inst.Playbook, &inst.Symbol, &cfg, &inst.ConfigHash,
		&inst.PlaybookKey, &inst.WorkspaceRelPath, &inst.Status, &inst.Error,
		&started, &finished, &reportJSON,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ExperimentInstance{}, fmt.Errorf("experiment instance not found")
	}
	if err != nil {
		return ExperimentInstance{}, err
	}
	inst.StrategyConfig = json.RawMessage(cfg)
	finishInstanceScan(&inst, started, finished, reportJSON)
	return inst, nil
}

func finishInstanceScan(inst *ExperimentInstance, started, finished sql.NullInt64, reportJSON string) {
	if started.Valid {
		v := started.Int64
		inst.StartedAt = &v
	}
	if finished.Valid {
		v := finished.Int64
		inst.FinishedAt = &v
	}
	if reportJSON != "" {
		inst.RunReportJSON = json.RawMessage(reportJSON)
	}
}

// normalizeExperimentInstanceConfig validates playbook strategy JSON and returns
// canonical JSON, config hash, and a short default label fragment.
func normalizeExperimentInstanceConfig(slug string, raw json.RawMessage, aggMS int64) (configJSON []byte, hash, labelFrag string, err error) {
	switch slug {
	case "ma-cross":
		cfg, err := macross.ParseConfig(raw)
		if err != nil {
			return nil, "", "", err
		}
		if err := cfg.Validate(aggMS); err != nil {
			return nil, "", "", err
		}
		configJSON, err = cfg.CanonicalJSON()
		if err != nil {
			return nil, "", "", err
		}
		hash, err = cfg.Hash()
		if err != nil {
			return nil, "", "", err
		}
		return configJSON, hash, fmt.Sprintf("%dh / %dh", cfg.FastHr, cfg.SlowHr), nil
	case "signal-consensus":
		cfg, err := signalconsensus.ParseConfig(raw)
		if err != nil {
			return nil, "", "", err
		}
		if err := cfg.Validate(aggMS); err != nil {
			return nil, "", "", err
		}
		configJSON, err = cfg.CanonicalJSON()
		if err != nil {
			return nil, "", "", err
		}
		hash, err = cfg.Hash()
		if err != nil {
			return nil, "", "", err
		}
		return configJSON, hash, fmt.Sprintf("rsi %.0f/%.0f atr×%.1f", cfg.RsiBull, cfg.RsiBear, cfg.AtrStopMult), nil
	default:
		return nil, "", "", fmt.Errorf("experiment instances do not support playbook %q (need StrategyConfig)", slug)
	}
}

// InstanceSymbol returns the instance pipeline symbol or experiment settings fallback.
func InstanceSymbol(inst ExperimentInstance, settings ExperimentSettings) string {
	if s := strings.TrimSpace(inst.Symbol); s != "" {
		return s
	}
	return settings.PipelineSymbol()
}

// MarkExperimentRunning sets experiment + instances to running.
func (c *Catalog) MarkExperimentRunning(id string) error {
	now := time.Now().UnixMilli()
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(
		`UPDATE experiment SET status = ?, started_at = ?, finished_at = NULL, error = '',
		 progress_stage = 1, progress_stages = ?, progress_stage_label = ?,
		 progress_instance_done = 0 WHERE id = ?`,
		StatusRunning, now, ExperimentStageTotal, ExperimentStageLabelPrepare, id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("experiment not found")
	}
	if _, err := tx.Exec(
		`UPDATE experiment_instance SET status = ?, error = '' WHERE experiment_id = ?`,
		StatusRunning, id,
	); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// MarkExperimentRetryFailed sets the experiment running and only resets previously failed instances.
// Completed (and other non-failed) instances keep their status.
func (c *Catalog) MarkExperimentRetryFailed(id string) (failedCount int, err error) {
	now := time.Now().UnixMilli()
	tx, err := c.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var nFail int
	if err := tx.QueryRow(
		`SELECT COUNT(*) FROM experiment_instance WHERE experiment_id = ? AND status = ?`,
		id, StatusFailed,
	).Scan(&nFail); err != nil {
		return 0, err
	}
	if nFail == 0 {
		return 0, fmt.Errorf("no failed instances to retry")
	}
	res, err := tx.Exec(
		`UPDATE experiment SET status = ?, started_at = ?, finished_at = NULL, error = '',
		 progress_stage = 1, progress_stages = ?, progress_stage_label = ?,
		 progress_instance_done = 0, progress_instance_total = ? WHERE id = ?`,
		StatusRunning, now, ExperimentStageTotal, ExperimentStageLabelPrepare, nFail, id,
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return 0, fmt.Errorf("experiment not found")
	}
	if _, err := tx.Exec(
		`UPDATE experiment_instance SET status = ?, error = '' WHERE experiment_id = ? AND status = ?`,
		StatusRunning, id, StatusFailed,
	); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return nFail, nil
}

// MarkExperimentRunningKeepCompleted sets the experiment running without changing completed instances.
// Used when a retry already flipped failed→running in the API.
func (c *Catalog) MarkExperimentRunningKeepCompleted(id string, retryTotal int) error {
	now := time.Now().UnixMilli()
	if retryTotal < 0 {
		retryTotal = 0
	}
	res, err := c.db.Exec(
		`UPDATE experiment SET status = ?, started_at = ?, finished_at = NULL, error = '',
		 progress_stage = 1, progress_stages = ?, progress_stage_label = ?,
		 progress_instance_done = 0, progress_instance_total = ? WHERE id = ?`,
		StatusRunning, now, ExperimentStageTotal, ExperimentStageLabelPrepare, retryTotal, id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("experiment not found")
	}
	return nil
}

// SetExperimentProgress updates coarse experiment progress (no bar-level spam).
func (c *Catalog) SetExperimentProgress(id string, stage int, label string, instanceDone, instanceTotal int) error {
	if stage < 1 {
		stage = 1
	}
	if stage > ExperimentStageTotal {
		stage = ExperimentStageTotal
	}
	if label == "" {
		label = ExperimentStageLabelPrepare
	}
	_, err := c.db.Exec(
		`UPDATE experiment SET progress_stage = ?, progress_stages = ?, progress_stage_label = ?,
		 progress_instance_done = ?, progress_instance_total = ? WHERE id = ?`,
		stage, ExperimentStageTotal, label, instanceDone, instanceTotal, id,
	)
	if err != nil {
		return err
	}
	return nil
}

// MarkExperimentInstanceCompleted marks one instance finished and records run timing when set.
func (c *Catalog) MarkExperimentInstanceCompleted(instanceID string, timing InstanceRunTiming) error {
	_, err := c.db.Exec(
		`UPDATE experiment_instance SET status = ?, error = '', started_at = ?, finished_at = ?, run_report_json = '' WHERE id = ?`,
		StatusCompleted, nullInt64(timing.StartedAtMs), nullInt64(timing.FinishedAtMs), instanceID,
	)
	if err != nil {
		return err
	}
	return nil
}

// MarkExperimentInstanceFailed marks one instance failed; timing is optional (zeros leave columns null).
func (c *Catalog) MarkExperimentInstanceFailed(instanceID, message string, timing InstanceRunTiming) error {
	_, err := c.db.Exec(
		`UPDATE experiment_instance SET status = ?, error = ?, started_at = ?, finished_at = ?, run_report_json = '' WHERE id = ?`,
		StatusFailed, message, nullInt64(timing.StartedAtMs), nullInt64(timing.FinishedAtMs), instanceID,
	)
	if err != nil {
		return err
	}
	return nil
}

func nullInt64(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

// MarkExperimentCompleted sets experiment completed.
func (c *Catalog) MarkExperimentCompleted(id string) error {
	now := time.Now().UnixMilli()
	_, err := c.db.Exec(
		`UPDATE experiment SET status = ?, finished_at = ?, error = '',
		 progress_stage = ?, progress_stages = ?, progress_stage_label = ? WHERE id = ?`,
		StatusCompleted, now, ExperimentStageTotal, ExperimentStageTotal, ExperimentStageLabelSimulation, id,
	)
	if err != nil {
		return err
	}
	return nil
}

// MarkExperimentFailed records experiment failure.
func (c *Catalog) MarkExperimentFailed(id, message string) error {
	now := time.Now().UnixMilli()
	_, err := c.db.Exec(
		`UPDATE experiment SET status = ?, finished_at = ?, error = ? WHERE id = ?`,
		StatusFailed, now, message, id,
	)
	if err != nil {
		return err
	}
	return nil
}

// MarkExperimentPartial records a finished experiment with a mix of completed and failed instances.
func (c *Catalog) MarkExperimentPartial(id, message string) error {
	now := time.Now().UnixMilli()
	_, err := c.db.Exec(
		`UPDATE experiment SET status = ?, finished_at = ?, error = ?,
		 progress_stage = ?, progress_stages = ?, progress_stage_label = ? WHERE id = ?`,
		StatusPartial, now, message, ExperimentStageTotal, ExperimentStageTotal, ExperimentStageLabelSimulation, id,
	)
	if err != nil {
		return err
	}
	return nil
}

// MarkExperimentCancelled stops a run in the catalog (job kill is separate).
func (c *Catalog) MarkExperimentCancelled(id string) error {
	now := time.Now().UnixMilli()
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`UPDATE experiment SET status = ?, finished_at = ?, error = ?, progress_stage_label = ? WHERE id = ?`,
		StatusCancelled, now, "cancelled", "Cancelled", id,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`UPDATE experiment_instance SET status = ?, error = CASE WHEN status = ? THEN 'cancelled' ELSE error END WHERE experiment_id = ?`,
		StatusCancelled, StatusRunning, id,
	); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// DeleteExperiment removes catalog rows and on-disk experiment workspace.
func (c *Catalog) DeleteExperiment(dataDir, id string) error {
	rec, err := c.GetExperiment(id)
	if err != nil {
		return err
	}
	if rec.Status == StatusRunning {
		return fmt.Errorf("cannot delete running experiment; cancel first")
	}
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM experiment_kpi WHERE experiment_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM experiment_instance WHERE experiment_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM experiment WHERE id = ?`, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	_ = os.RemoveAll(StudyDir(dataDir, id))
	return nil
}

// ExperimentWorkspaceDir resolves absolute path for an instance workspace.
func ExperimentWorkspaceDir(dataDir string, inst ExperimentInstance) string {
	return filepath.Join(dataDir, inst.WorkspaceRelPath)
}

// StudyDir resolves absolute experiment root under dataDir.
func StudyDir(dataDir, experimentID string) string {
	return filepath.Join(dataDir, "experiments", experimentID)
}
