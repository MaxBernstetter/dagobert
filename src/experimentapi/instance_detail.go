package experimentapi

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"strings"

	"connectrpc.com/connect"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/MaxBernstetter/dagobert/src/internal/instanceparquet"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/structpb"
)

// The instance detail methods read the shard written by the backtest job: the instance's report
// (KPIs and statistics) and its row groups in the shard Parquet files (orders, fills, journals).
// An instance without shard_key (not completed) reads like a missing file.

// loadInstance loads the instance and checks that it belongs to the experiment.
func (s *Server) loadInstance(ctx context.Context, rawExperimentID, rawInstanceID string) (experimentdb.ExperimentInstance, error) {
	experimentID, err := parseExperimentID(rawExperimentID)
	if err != nil {
		return experimentdb.ExperimentInstance{}, invalidArgument("invalid id")
	}
	instID, err := uuid.Parse(rawInstanceID)
	if err != nil {
		return experimentdb.ExperimentInstance{}, invalidArgument("invalid instance id")
	}
	inst, err := s.q.GetInstance(ctx, experimentdb.GetInstanceParams{ExperimentID: experimentID, ID: instID})
	if errors.Is(err, pgx.ErrNoRows) {
		return experimentdb.ExperimentInstance{}, notFound("not found")
	}
	if err != nil {
		return experimentdb.ExperimentInstance{}, internal(err)
	}
	return inst, nil
}

// dataDir is the workspace key root (DATA_DIR); shard keys are relative to it.
func dataDir() string {
	if d := strings.TrimSpace(os.Getenv("DATA_DIR")); d != "" {
		return d
	}
	return "/data"
}

// instanceShard is the shard key of the instance. The error satisfies errors.Is(err, fs.ErrNotExist)
// when the instance has no shard.
func instanceShard(inst experimentdb.ExperimentInstance) (string, error) {
	if inst.ShardKey == "" {
		return "", fs.ErrNotExist
	}
	return inst.ShardKey, nil
}

// readInstanceReport is the report JSON of the instance as stored in its shard.
func readInstanceReport(ctx context.Context, inst experimentdb.ExperimentInstance) ([]byte, error) {
	dir, err := instanceShard(inst)
	if err != nil {
		return nil, err
	}
	return instanceparquet.ReadReport(ctx, dataDir(), dir, inst.ID.String())
}

func (s *Server) GetInstanceReport(ctx context.Context, req *connect.Request[v1.GetInstanceReportRequest]) (*connect.Response[v1.GetInstanceReportResponse], error) {
	inst, err := s.loadInstance(ctx, req.Msg.ExperimentId, req.Msg.InstanceId)
	if err != nil {
		return nil, err
	}
	raw, err := readInstanceReport(ctx, inst)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, notFound("report not found")
	}
	if err != nil {
		return nil, internal(err)
	}
	report := &structpb.Struct{}
	if err := report.UnmarshalJSON(raw); err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&v1.GetInstanceReportResponse{Report: report}), nil
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

func readDetailReport(ctx context.Context, inst experimentdb.ExperimentInstance) (detailReport, error) {
	var report detailReport
	raw, err := readInstanceReport(ctx, inst)
	if err != nil {
		return report, err
	}
	err = json.Unmarshal(raw, &report)
	return report, err
}

func (s *Server) GetInstanceKpis(ctx context.Context, req *connect.Request[v1.GetInstanceKpisRequest]) (*connect.Response[v1.GetInstanceKpisResponse], error) {
	inst, err := s.loadInstance(ctx, req.Msg.ExperimentId, req.Msg.InstanceId)
	if err != nil {
		return nil, err
	}
	report, err := readDetailReport(ctx, inst)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && report.KPIs == nil) {
		return nil, notFound("kpis not found")
	}
	if err != nil {
		return nil, internal(err)
	}
	stats := report.Statistics
	if stats == nil {
		stats = &detailStats{}
	}
	results := make([]*v1.KpiResult, 0, len(report.KPIs.Results))
	for _, res := range report.KPIs.Results {
		value, err := jsonValue(res.Value)
		if err != nil {
			return nil, internal(err)
		}
		extra, err := jsonValue(res.Extra)
		if err != nil {
			return nil, internal(err)
		}
		results = append(results, &v1.KpiResult{
			Def: &v1.KpiDef{
				Slug:   res.Def.Slug,
				Title:  defaultString(res.Def.Title, res.Def.Slug),
				Label:  defaultString(res.Def.Label, res.Def.Slug),
				Kind:   defaultString(res.Def.Kind, "strategy-kpi"),
				Unit:   res.Def.Unit,
				Format: defaultString(res.Def.Format, "decimal"),
			},
			Value: value,
			Extra: extra,
		})
	}
	comparison, err := jsonValue(report.KPIs.Comparison)
	if err != nil {
		return nil, internal(err)
	}
	run, err := jsonValue(report.Run)
	if err != nil {
		return nil, internal(err)
	}
	monthly, err := jsonValues(stats.MonthlyReturns)
	if err != nil {
		return nil, internal(err)
	}
	yearly, err := jsonValues(stats.YearlyReturns)
	if err != nil {
		return nil, internal(err)
	}
	resp := &v1.KpiReport{
		Playbook:   report.KPIs.Playbook,
		Symbol:     report.KPIs.Symbol,
		FromMs:     report.KPIs.FromMs,
		ToMs:       report.KPIs.ToMs,
		Results:    results,
		Comparison: comparison,
		Statistics: &v1.KpiStatistics{
			DurationMonths:    stats.DurationMonths,
			DurationYears:     stats.DurationYears,
			MonthsSampled:     int32(len(stats.MonthlyReturns)),
			YearsSampled:      int32(len(stats.YearlyReturns)),
			MonthlyReturnAvg:  averageOf(stats.MonthlyReturn, false),
			MonthlyReturnMed:  averageOf(stats.MonthlyReturn, true),
			YearlyReturnAvg:   averageOf(stats.YearlyReturn, false),
			YearlyReturnMed:   averageOf(stats.YearlyReturn, true),
			TradesPerMonthAvg: averageOf(stats.TradesPerMonth, false),
			TradesPerMonthMed: averageOf(stats.TradesPerMonth, true),
			MonthlyReturns:    monthly,
			YearlyReturns:     yearly,
		},
		Run: run,
	}
	if err := s.attachInstancePerformance(ctx, inst, resp); err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&v1.GetInstanceKpisResponse{Kpis: resp}), nil
}

// jsonValue converts stored JSON to a protobuf Value; an absent value stays nil.
func jsonValue(raw json.RawMessage) (*structpb.Value, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	v := &structpb.Value{}
	if err := v.UnmarshalJSON(raw); err != nil {
		return nil, err
	}
	return v, nil
}

func jsonValues(raws []json.RawMessage) ([]*structpb.Value, error) {
	out := make([]*structpb.Value, 0, len(raws))
	for _, raw := range raws {
		v, err := jsonValue(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
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

// GetInstanceActivity returns the order results and fills from the instance's shard row groups.
// An instance without orders has no activity (empty lists).
func (s *Server) GetInstanceActivity(ctx context.Context, req *connect.Request[v1.GetInstanceActivityRequest]) (*connect.Response[v1.GetInstanceActivityResponse], error) {
	inst, err := s.loadInstance(ctx, req.Msg.ExperimentId, req.Msg.InstanceId)
	if err != nil {
		return nil, err
	}
	resp := &v1.GetInstanceActivityResponse{
		PlaybookTitles: map[string]string{inst.PlaybookKey: inst.Label},
		ExperimentId:   inst.ExperimentID.String(),
		InstanceId:     inst.ID.String(),
		Playbook:       inst.PlaybookKey,
		Symbol:         inst.Symbol,
	}
	ws, err := instanceShard(inst)
	var orders []instanceparquet.Order
	if err == nil {
		orders, err = instanceparquet.ReadOrders(ctx, dataDir(), ws, inst.ID.String())
	}
	if errors.Is(err, fs.ErrNotExist) {
		return connect.NewResponse(resp), nil
	}
	if err != nil {
		return nil, internal(err)
	}
	resp.Rows = activityRowsFromOrders(orders)
	journals, err := instanceparquet.ReadJournals(ctx, dataDir(), ws, inst.ID.String())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, internal(err)
	}
	if resp.Journals = journalRowsFromParquet(journals); len(resp.Journals) == 0 {
		resp.Journals = journalsFromFillOrders(orders)
	}
	return connect.NewResponse(resp), nil
}
