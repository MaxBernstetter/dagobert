package store

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// Experiment types. The experiment catalog itself is Postgres (src/experimentapi/pgcatalog); these are
// the shared value types the run, the API tests and the legacy backtest path use.

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
	// Parallelism is the C++ sweep process count; 0 = auto (one per CPU).
	Parallelism int `json:"parallelism,omitempty"`
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

// InstanceListMetrics are denormalized scalars for experiment instance table sort/filter.
// Nil pointers mean "not yet available" (SQL NULL).
type InstanceListMetrics struct {
	DurationMs        *int64
	Cagr              *float64
	MaxDrawdown       *float64
	Sharpe            *float64
	Sortino           *float64
	MonthlyReturnAvg  *float64
	MonthlyReturnMed  *float64
	YearlyReturnAvg   *float64
	YearlyReturnMed   *float64
	TradesPerMonthAvg *float64
	TradesPerMonthMed *float64
	Wins              *int32
	Losses            *int32
	Breakeven         *int32
	WinRate           *float64
	MonthlyWinRateAvg *float64
	MonthlyWinRateMed *float64
	YearlyWinRateAvg  *float64
	YearlyWinRateMed  *float64
}

// InstanceBatchItem is the final state of one instance for a batched catalog write.
// A non-empty Error marks the instance failed; otherwise it is completed with Metrics.
type InstanceBatchItem struct {
	InstanceID string
	Error      string
	Timing     InstanceRunTiming
	Metrics    InstanceListMetrics
}

// InstanceSymbol returns the instance pipeline symbol or experiment settings fallback.
func InstanceSymbol(inst ExperimentInstance, settings ExperimentSettings) string {
	if s := strings.TrimSpace(inst.Symbol); s != "" {
		return s
	}
	return settings.PipelineSymbol()
}

// ExperimentWorkspaceDir resolves absolute path for an instance workspace.
func ExperimentWorkspaceDir(dataDir string, inst ExperimentInstance) string {
	return filepath.Join(dataDir, inst.WorkspaceRelPath)
}

// StudyDir resolves absolute experiment root under dataDir.
func StudyDir(dataDir, experimentID string) string {
	return filepath.Join(dataDir, "experiments", experimentID)
}
