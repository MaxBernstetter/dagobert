package experimentapi

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	"github.com/MaxBernstetter/dagobert/src/internal/instanceparquet"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// The instance detail endpoints read the shard written by the backtest job: the instance's report
// (KPIs and statistics) and its row groups in the shard Parquet files (orders, fills, journals).
// An instance without shard_key (not completed) reads like a missing file.

// instanceForRequest loads the instance named in the path and checks that it belongs to the experiment.
func (s *Server) instanceForRequest(w http.ResponseWriter, r *http.Request) (experimentdb.ExperimentInstance, bool) {
	experimentID, err := parseExperimentID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return experimentdb.ExperimentInstance{}, false
	}
	instID, err := uuid.Parse(r.PathValue("instanceId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid instance id")
		return experimentdb.ExperimentInstance{}, false
	}
	inst, err := s.q.GetInstance(r.Context(), experimentdb.GetInstanceParams{ExperimentID: experimentID, ID: instID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not found")
		return experimentdb.ExperimentInstance{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return experimentdb.ExperimentInstance{}, false
	}
	return inst, true
}

// instanceShard is the shard directory of the instance under DATA_DIR. The error satisfies
// errors.Is(err, fs.ErrNotExist) when the instance has no shard.
func instanceShard(inst experimentdb.ExperimentInstance) (string, error) {
	if inst.ShardKey == "" {
		return "", fs.ErrNotExist
	}
	root := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if root == "" {
		root = "/data"
	}
	return filepath.Join(root, inst.ShardKey), nil
}

// readInstanceReport is the report JSON of the instance as stored in its shard.
func readInstanceReport(inst experimentdb.ExperimentInstance) ([]byte, error) {
	dir, err := instanceShard(inst)
	if err != nil {
		return nil, err
	}
	return instanceparquet.ReadReport(dir, inst.ID.String())
}

// handleInstanceReport serves GET .../instances/{instanceId}/report: the report JSON as stored in the shard.
func (s *Server) handleInstanceReport(w http.ResponseWriter, r *http.Request) {
	inst, ok := s.instanceForRequest(w, r)
	if !ok {
		return
	}
	raw, err := readInstanceReport(inst)
	if errors.Is(err, fs.ErrNotExist) {
		writeError(w, http.StatusNotFound, "report not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(raw)
}

type detailReport struct {
	Run        json.RawMessage `json:"run"`
	KPIs       *detailKPIs     `json:"kpis"`
	Statistics *detailStats    `json:"statistics"`
}

type detailKPIs struct {
	Playbook   string            `json:"playbook"`
	Symbol     string            `json:"symbol"`
	FromMs     int64             `json:"fromMs"`
	ToMs       int64             `json:"toMs"`
	Results    []detailKPIResult `json:"results"`
	Comparison json.RawMessage   `json:"comparison"`
}

type detailKPIResult struct {
	Def   detailKPIDef    `json:"def"`
	Value json.RawMessage `json:"value"`
	Extra json.RawMessage `json:"extra"`
}

type detailKPIDef struct {
	Slug   string `json:"slug"`
	Title  string `json:"title"`
	Label  string `json:"label"`
	Kind   string `json:"kind"`
	Unit   string `json:"unit"`
	Format string `json:"format"`
}

type detailAverages struct {
	Average *float64 `json:"average"`
	Median  *float64 `json:"median"`
}

type detailStats struct {
	DurationMonths float64           `json:"durationMonths"`
	DurationYears  float64           `json:"durationYears"`
	MonthlyReturn  *detailAverages   `json:"monthlyReturn"`
	YearlyReturn   *detailAverages   `json:"yearlyReturn"`
	TradesPerMonth *detailAverages   `json:"tradesPerMonth"`
	MonthlyReturns []json.RawMessage `json:"monthlyReturns"`
	YearlyReturns  []json.RawMessage `json:"yearlyReturns"`
}

func readDetailReport(inst experimentdb.ExperimentInstance) (detailReport, error) {
	var report detailReport
	raw, err := readInstanceReport(inst)
	if err != nil {
		return report, err
	}
	err = json.Unmarshal(raw, &report)
	return report, err
}

type kpisResponse struct {
	Playbook   string          `json:"playbook"`
	Symbol     string          `json:"symbol"`
	FromMs     int64           `json:"fromMs"`
	ToMs       int64           `json:"toMs"`
	Results    []kpisResult    `json:"results"`
	Comparison json.RawMessage `json:"comparison"`
	Statistics kpisStatistics  `json:"statistics"`
	Run        json.RawMessage `json:"run"`
}

type kpisResult struct {
	Def   kpisDef         `json:"def"`
	Value json.RawMessage `json:"value"`
	Extra json.RawMessage `json:"extra,omitempty"`
}

type kpisDef struct {
	Slug   string `json:"slug"`
	Title  string `json:"title"`
	Label  string `json:"label"`
	Kind   string `json:"kind"`
	Unit   string `json:"unit,omitempty"`
	Format string `json:"format"`
}

type kpisStatistics struct {
	DurationMonths    float64           `json:"durationMonths"`
	DurationYears     float64           `json:"durationYears"`
	MonthsSampled     int               `json:"monthsSampled"`
	YearsSampled      int               `json:"yearsSampled"`
	MonthlyReturnAvg  *float64          `json:"monthlyReturnAvg"`
	MonthlyReturnMed  *float64          `json:"monthlyReturnMed"`
	YearlyReturnAvg   *float64          `json:"yearlyReturnAvg"`
	YearlyReturnMed   *float64          `json:"yearlyReturnMed"`
	TradesPerMonthAvg *float64          `json:"tradesPerMonthAvg"`
	TradesPerMonthMed *float64          `json:"tradesPerMonthMed"`
	MonthlyReturns    []json.RawMessage `json:"monthlyReturns"`
	YearlyReturns     []json.RawMessage `json:"yearlyReturns"`
	WinRate           *float64          `json:"winRate,omitempty"`
	MonthlyWinRateAvg *float64          `json:"monthlyWinRateAvg,omitempty"`
	MonthlyWinRateMed *float64          `json:"monthlyWinRateMed,omitempty"`
	YearlyWinRateAvg  *float64          `json:"yearlyWinRateAvg,omitempty"`
	YearlyWinRateMed  *float64          `json:"yearlyWinRateMed,omitempty"`
	Gain              *outcomeSide      `json:"gain,omitempty"`
	Loss              *outcomeSide      `json:"loss,omitempty"`
}

// handleInstanceKPIs serves GET .../instances/{instanceId}/kpis: KPIs, comparison, and statistics from the report.
func (s *Server) handleInstanceKPIs(w http.ResponseWriter, r *http.Request) {
	inst, ok := s.instanceForRequest(w, r)
	if !ok {
		return
	}
	report, err := readDetailReport(inst)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && report.KPIs == nil) {
		writeError(w, http.StatusNotFound, "kpis not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	stats := report.Statistics
	if stats == nil {
		stats = &detailStats{}
	}
	results := make([]kpisResult, 0, len(report.KPIs.Results))
	for _, res := range report.KPIs.Results {
		results = append(results, kpisResult{
			Def: kpisDef{
				Slug:   res.Def.Slug,
				Title:  defaultString(res.Def.Title, res.Def.Slug),
				Label:  defaultString(res.Def.Label, res.Def.Slug),
				Kind:   defaultString(res.Def.Kind, "strategy-kpi"),
				Unit:   res.Def.Unit,
				Format: defaultString(res.Def.Format, "decimal"),
			},
			Value: res.Value,
			Extra: res.Extra,
		})
	}
	resp := kpisResponse{
		Playbook:   report.KPIs.Playbook,
		Symbol:     report.KPIs.Symbol,
		FromMs:     report.KPIs.FromMs,
		ToMs:       report.KPIs.ToMs,
		Results:    results,
		Comparison: report.KPIs.Comparison,
		Statistics: kpisStatistics{
			DurationMonths:    stats.DurationMonths,
			DurationYears:     stats.DurationYears,
			MonthsSampled:     len(stats.MonthlyReturns),
			YearsSampled:      len(stats.YearlyReturns),
			MonthlyReturnAvg:  averageOf(stats.MonthlyReturn, false),
			MonthlyReturnMed:  averageOf(stats.MonthlyReturn, true),
			YearlyReturnAvg:   averageOf(stats.YearlyReturn, false),
			YearlyReturnMed:   averageOf(stats.YearlyReturn, true),
			TradesPerMonthAvg: averageOf(stats.TradesPerMonth, false),
			TradesPerMonthMed: averageOf(stats.TradesPerMonth, true),
			MonthlyReturns:    nonNilRaw(stats.MonthlyReturns),
			YearlyReturns:     nonNilRaw(stats.YearlyReturns),
		},
		Run: report.Run,
	}
	if err := s.attachInstancePerformance(r.Context(), inst, &resp); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func defaultString(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func averageOf(a *detailAverages, median bool) *float64 {
	if a == nil {
		return nil
	}
	if median {
		return a.Median
	}
	return a.Average
}

func nonNilRaw(v []json.RawMessage) []json.RawMessage {
	if v == nil {
		return []json.RawMessage{}
	}
	return v
}

// activityRow mirrors the activity row the Experiment UI renders. Signal and action fields stay null:
// experiment instances record orders and fills only.
type activityRow struct {
	OrderLinkID    string   `json:"orderLinkId"`
	Playbook       string   `json:"playbook"`
	Symbol         string   `json:"symbol"`
	CandleClose    int64    `json:"candleClose"`
	Kind           string   `json:"kind"`
	SignalDecision *string  `json:"signalDecision"`
	SignalRule     *string  `json:"signalRule"`
	Decision       *string  `json:"decision"`
	Rule           *string  `json:"rule"`
	Size           *float64 `json:"size"`
	Exit           any      `json:"exit"`
	WrittenAt      *string  `json:"writtenAt"`
	OrderID        *string  `json:"orderId"`
	OrderStatus    *string  `json:"orderStatus"`
	VenueSymbol    *string  `json:"venueSymbol"`
	Side           *string  `json:"side"`
	Qty            *string  `json:"qty"`
	CumExecQty     *string  `json:"cumExecQty"`
	CumExecValue   *string  `json:"cumExecValue"`
	AvgPrice       *string  `json:"avgPrice"`
	MarketUnit     *string  `json:"marketUnit"`
	RetCode        *float64 `json:"retCode"`
	RetMsg         *string  `json:"retMsg"`
}

type journalRow struct {
	OrderLinkID string `json:"orderLinkId"`
	Venue       string `json:"venue"`
	VenueSymbol string `json:"venueSymbol"`
	Side        string `json:"side"`
	BaseQty     string `json:"baseQty"`
	QuoteQty    string `json:"quoteQty"`
	Price       string `json:"price"`
	Fee         string `json:"fee"`
	FeeCurrency string `json:"feeCurrency"`
	PriceSource string `json:"priceSource"`
}

type activityResponse struct {
	Rows           []activityRow     `json:"rows"`
	Journals       []journalRow      `json:"journals"`
	PlaybookTitles map[string]string `json:"playbookTitles"`
	ExperimentID   string            `json:"experimentId"`
	InstanceID     string            `json:"instanceId"`
	Playbook       string            `json:"playbook"`
	Symbol         string            `json:"symbol"`
}

// handleInstanceActivity serves GET .../instances/{instanceId}/activity: order results and fills from the
// instance's shard row groups. An instance without orders has no activity (empty arrays).
func (s *Server) handleInstanceActivity(w http.ResponseWriter, r *http.Request) {
	inst, ok := s.instanceForRequest(w, r)
	if !ok {
		return
	}
	resp := activityResponse{
		Rows:           []activityRow{},
		Journals:       []journalRow{},
		PlaybookTitles: map[string]string{inst.PlaybookKey: inst.Label},
		ExperimentID:   inst.ExperimentID.String(),
		InstanceID:     inst.ID.String(),
		Playbook:       inst.PlaybookKey,
		Symbol:         inst.Symbol,
	}
	ws, err := instanceShard(inst)
	var orders []instanceparquet.Order
	if err == nil {
		orders, err = instanceparquet.ReadOrders(ws, inst.ID.String())
	}
	if errors.Is(err, fs.ErrNotExist) {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Rows = activityRowsFromOrders(orders)
	journals, err := instanceparquet.ReadJournals(ws, inst.ID.String())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if resp.Journals = journalRowsFromParquet(journals); len(resp.Journals) == 0 {
		resp.Journals = journalsFromFillOrders(orders)
	}
	writeJSON(w, http.StatusOK, resp)
}
