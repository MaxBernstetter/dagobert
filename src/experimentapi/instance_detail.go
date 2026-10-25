package experimentapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	_ "modernc.org/sqlite"
)

// The instance detail endpoints read the instance workspace written by the backtest job:
// report.json (KPIs and statistics) and execution.sqlite (orders and fills).

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

// instanceWorkspace is the directory of one instance under DATA_DIR.
func instanceWorkspace(inst experimentdb.ExperimentInstance) string {
	root := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if root == "" {
		root = "/data"
	}
	return filepath.Join(root, inst.WorkspaceRelPath)
}

// handleInstanceReport serves GET .../instances/{instanceId}/report: the report.json file as written.
func (s *Server) handleInstanceReport(w http.ResponseWriter, r *http.Request) {
	inst, ok := s.instanceForRequest(w, r)
	if !ok {
		return
	}
	raw, err := os.ReadFile(filepath.Join(instanceWorkspace(inst), "report.json"))
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
	raw, err := os.ReadFile(filepath.Join(instanceWorkspace(inst), "report.json"))
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

// handleInstanceKPIs serves GET .../instances/{instanceId}/kpis: KPIs, comparison, and statistics from report.json.
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
	if err := s.attachInstancePerformance(r.Context(), inst.ID, &resp); err != nil {
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

// handleInstanceActivity serves GET .../instances/{instanceId}/activity: order results and fills from execution.sqlite.
func (s *Server) handleInstanceActivity(w http.ResponseWriter, r *http.Request) {
	inst, ok := s.instanceForRequest(w, r)
	if !ok {
		return
	}
	db, err := openReadOnlySQLite(filepath.Join(instanceWorkspace(inst), "execution.sqlite"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
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
	if db != nil {
		defer db.Close()
		if resp.Rows, err = readActivityRows(db); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if resp.Journals, err = readJournalRows(db); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// openReadOnlySQLite opens a file that must already exist. A missing file returns nil, not an error.
func openReadOnlySQLite(path string) (*sql.DB, error) {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return sql.Open("sqlite", "file:"+path+"?mode=ro")
}

func readActivityRows(db *sql.DB) ([]activityRow, error) {
	rows := []activityRow{}
	res, err := db.Query(`SELECT order_link_id, kind, body FROM order_result`)
	if err != nil {
		return rows, nil // no order_result table yet: the instance has no orders
	}
	defer res.Close()
	for res.Next() {
		var linkID, kind, rawBody string
		if err := res.Scan(&linkID, &kind, &rawBody); err != nil {
			return nil, err
		}
		body := map[string]any{}
		_ = json.Unmarshal([]byte(rawBody), &body)
		playbook, symbol, candleClose := keysFromOrder(body, linkID)
		rows = append(rows, activityRow{
			OrderLinkID:  linkID,
			Playbook:     playbook,
			Symbol:       symbol,
			CandleClose:  candleClose,
			Kind:         kind,
			Decision:     strFrom(body, "decision"),
			Size:         numFrom(body, "size"),
			OrderID:      strFrom(body, "orderId"),
			OrderStatus:  strFrom(body, "orderStatus"),
			VenueSymbol:  strFrom(body, "venueSymbol"),
			Side:         strFrom(body, "side"),
			Qty:          strFrom(body, "qty"),
			CumExecQty:   strFrom(body, "cumExecQty"),
			CumExecValue: strFrom(body, "cumExecValue"),
			AvgPrice:     strFrom(body, "avgPrice"),
			MarketUnit:   strFrom(body, "marketUnit"),
			RetCode:      numFrom(body, "retCode"),
			RetMsg:       strFrom(body, "retMsg"),
		})
	}
	if err := res.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].CandleClose > rows[j].CandleClose })
	return rows, nil
}

func readJournalRows(db *sql.DB) ([]journalRow, error) {
	journals := []journalRow{}
	var hasJournal int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'fill_journal'`).Scan(&hasJournal); err != nil {
		return nil, err
	}
	if hasJournal > 0 {
		res, err := db.Query(`SELECT COALESCE(order_link_id,''), COALESCE(venue,''), COALESCE(venue_symbol,''), COALESCE(side,''),
			COALESCE(base_qty,''), COALESCE(quote_qty,''), COALESCE(price,''), COALESCE(fee,''), COALESCE(fee_currency,''), COALESCE(price_source,'')
			FROM fill_journal`)
		if err != nil {
			return nil, err
		}
		defer res.Close()
		for res.Next() {
			var j journalRow
			if err := res.Scan(&j.OrderLinkID, &j.Venue, &j.VenueSymbol, &j.Side, &j.BaseQty, &j.QuoteQty, &j.Price, &j.Fee, &j.FeeCurrency, &j.PriceSource); err != nil {
				return nil, err
			}
			journals = append(journals, j)
		}
		if err := res.Err(); err != nil {
			return nil, err
		}
		if len(journals) > 0 {
			return journals, nil
		}
	}
	// Without a journal, derive the fills from the filled order results.
	res, err := db.Query(`SELECT body FROM order_result WHERE kind = 'fill'`)
	if err != nil {
		return journals, nil
	}
	defer res.Close()
	for res.Next() {
		var rawBody string
		if err := res.Scan(&rawBody); err != nil {
			return nil, err
		}
		body := map[string]any{}
		_ = json.Unmarshal([]byte(rawBody), &body)
		journals = append(journals, journalRow{
			OrderLinkID: strOr(body, "orderLinkId"),
			Venue:       "bybit",
			VenueSymbol: strOr(body, "venueSymbol"),
			Side:        strOr(body, "side"),
			BaseQty:     strOr(body, "cumExecQty"),
			QuoteQty:    strOr(body, "cumExecValue"),
			Price:       strOr(body, "avgPrice"),
			Fee:         strOr(body, "cumExecFee"),
			FeeCurrency: defaultString(strOr(body, "feeCurrency"), "USDC"),
			PriceSource: "binance",
		})
	}
	return journals, res.Err()
}

// keysFromOrder reads playbook, symbol, and candle close from the order body, falling back to the
// order link id, which is "<playbook>:<symbol>:<candleClose>".
func keysFromOrder(body map[string]any, linkID string) (string, string, int64) {
	playbook, symbol := strOr(body, "playbook"), strOr(body, "symbol")
	candle, candleOK := body["candleClose"].(float64)
	if playbook != "" && symbol != "" && candleOK {
		return playbook, symbol, int64(candle)
	}
	last := strings.LastIndex(linkID, ":")
	if last > 0 {
		if n, err := strconv.ParseInt(linkID[last+1:], 10, 64); err == nil {
			rest := linkID[:last]
			if sep := strings.LastIndex(rest, ":"); sep > 0 {
				return rest[:sep], rest[sep+1:], n
			}
		}
	}
	return playbook, symbol, int64(candle)
}

func strOr(body map[string]any, key string) string {
	v, ok := body[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func strFrom(body map[string]any, key string) *string {
	v, ok := body[key]
	if !ok || v == nil {
		return nil
	}
	s := fmt.Sprint(v)
	return &s
}

func numFrom(body map[string]any, key string) *float64 {
	v, ok := body[key].(float64)
	if !ok {
		return nil
	}
	return &v
}
