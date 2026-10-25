package experimentapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/MaxBernstetter/dagobert/src/internal/lakeobject"
	"github.com/MaxBernstetter/dagobert/src/shared"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/parquet-go/parquet-go"
	"google.golang.org/protobuf/proto"
)

var errChartParquetMissing = errors.New("parquet missing")

type chartCandle struct {
	OpenTime int64
	Open     float64
	High     float64
	Low      float64
	Close    float64
}

type chartIndicatorMeta struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// Unit is the TI result unit ("price", "index", "ratio", "amount"). Non-price indicators get their own pane.
	Unit string `json:"unit"`
	// Pane is "price" (drawn with the candles) or "oscillator" (its own pane).
	Pane string `json:"pane"`
	// Style is "line" or "histogram".
	Style string `json:"style"`
	// Reference lines drawn across the pane (e.g. RSI 30/70, MACD zero).
	Reference []float64 `json:"reference,omitempty"`
}

var marketSeriesCache = struct {
	mu sync.Mutex
	m  map[string]*v1.ChartSeries
}{m: map[string]*v1.ChartSeries{}}

func (s *Server) handleChartSeries(w http.ResponseWriter, r *http.Request) {
	experimentID, err := parseExperimentID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	instID, err := uuid.Parse(r.PathValue("instanceId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid instance id")
		return
	}
	fromSec, toSec, ok := chartWindow(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "from and to (unix seconds) required")
		return
	}
	exp, err := s.q.GetExperiment(r.Context(), experimentID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	inst, err := s.q.GetInstance(r.Context(), experimentdb.GetInstanceParams{ExperimentID: experimentID, ID: instID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	series, err := marketSeries(r.Context(), exp, fromSec, toSec)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	points, markers := instanceChartSeries(inst, exp.InitialUsdc+exp.InitialUsdt)
	series.Portfolio = points
	series.Markers = markers
	writeProtoBinary(w, http.StatusOK, series)
}

func chartWindow(r *http.Request) (fromSec, toSec int64, ok bool) {
	q := r.URL.Query()
	if q.Get("from") == "" || q.Get("to") == "" {
		return 0, 0, false
	}
	from, err1 := strconv.ParseInt(q.Get("from"), 10, 64)
	to, err2 := strconv.ParseInt(q.Get("to"), 10, 64)
	if err1 != nil || err2 != nil || from > to {
		return 0, 0, false
	}
	return from, to, true
}

func loadCandleWindow(ctx context.Context, dataDir, provider, base, quote string, fromSec, toSec int64) ([]chartCandle, error) {
	paths, err := lakeobject.ListCandleParquetAbsPathsInRange(ctx, dataDir, provider, base, quote, fromSec*1000, toSec*1000+1)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, errChartParquetMissing
	}
	var all []chartCandle
	found := false
	for _, path := range paths {
		part, err := readChartCandleFile(path)
		if errors.Is(err, errChartParquetMissing) {
			continue
		}
		if err != nil {
			return nil, err
		}
		found = true
		all = append(all, part...)
	}
	if !found {
		return nil, errChartParquetMissing
	}
	return all, nil
}

type parquetCandleRow struct {
	OpenTime int64   `parquet:"open_time"`
	Open     float64 `parquet:"open"`
	High     float64 `parquet:"high"`
	Low      float64 `parquet:"low"`
	Close    float64 `parquet:"close"`
}

func readChartCandleFile(path string) (candles []chartCandle, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("parquet read: %v", recovered)
		}
	}()
	file, size, err := lakeobject.OpenAbs(context.Background(), path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errChartParquetMissing
		}
		return nil, err
	}
	defer file.Close()
	pf, err := parquet.OpenFile(file.File, size)
	if err != nil {
		return nil, err
	}
	reader := parquet.NewGenericReader[parquetCandleRow](pf)
	defer reader.Close()
	buf := make([]parquetCandleRow, 256)
	for {
		n, readErr := reader.Read(buf)
		for _, item := range buf[:n] {
			candles = append(candles, chartCandle{
				OpenTime: item.OpenTime,
				Open:     item.Open,
				High:     item.High,
				Low:      item.Low,
				Close:    item.Close,
			})
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return candles, nil
			}
			return nil, readErr
		}
	}
}

func loadIndicatorWindow(dataDir, provider, base, quote, algo string, fromSec, toSec int64) (map[int64]map[string]float64, []chartIndicatorMeta) {
	fromMs := fromSec * 1000
	toMs := toSec * 1000
	byOpen := map[int64]map[string]float64{}
	metaSeen := map[string]chartIndicatorMeta{}
	for _, year := range shared.YearsSpanning(fromMs, toMs+1) {
		rows, err := readTIYear(dataDir, provider, base, quote, algo, year)
		if err != nil || len(rows) == 0 {
			continue
		}
		for _, row := range rows {
			if row.BarOpen < fromMs || row.BarOpen > toMs {
				continue
			}
			vals := numericIndicators(row.ResultsJSON, metaSeen)
			if vals == nil {
				vals = map[string]float64{}
			}
			byOpen[row.BarOpen] = vals
		}
	}
	ids := make([]string, 0, len(metaSeen))
	for id := range metaSeen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	meta := make([]chartIndicatorMeta, 0, len(ids))
	for _, id := range ids {
		meta = append(meta, metaSeen[id])
	}
	return byOpen, meta
}

func numericIndicators(resultsJSON string, meta map[string]chartIndicatorMeta) map[string]float64 {
	if strings.TrimSpace(resultsJSON) == "" {
		return map[string]float64{}
	}
	var raw []struct {
		Def   json.RawMessage `json:"def"`
		Value float64         `json:"value"`
		Unit  string          `json:"unit"`
	}
	if err := json.Unmarshal([]byte(resultsJSON), &raw); err != nil {
		return map[string]float64{}
	}
	out := map[string]float64{}
	for _, item := range raw {
		var def struct {
			Type        string `json:"type"`
			Method      string `json:"method"`
			TimeFrameHr int    `json:"time_frame_hr"`
		}
		if len(item.Def) > 0 && item.Def[0] == '"' {
			var inner string
			if err := json.Unmarshal(item.Def, &inner); err == nil {
				_ = json.Unmarshal([]byte(inner), &def)
			}
		} else {
			_ = json.Unmarshal(item.Def, &def)
		}
		id, label := indicatorID(def.Method, def.Type, def.TimeFrameHr)
		if id == "" {
			continue
		}
		out[id] = item.Value
		if _, ok := meta[id]; !ok {
			meta[id] = chartIndicatorMeta{
				ID:        id,
				Label:     label,
				Unit:      item.Unit,
				Pane:      indicatorPane(item.Unit, def.Type),
				Style:     indicatorStyle(def.Type),
				Reference: referenceLevels(def.Type),
			}
		}
	}
	return out
}

type tiParquetRow struct {
	BarOpen     int64  `parquet:"bar_open"`
	ResultsJSON string `parquet:"results_json"`
}

func readTIYear(dataDir, provider, base, quote, algo string, year int) ([]tiParquetRow, error) {
	if err := shared.ValidatePartitionValue(provider); err != nil {
		return nil, err
	}
	market, err := shared.NormalizeMarket(base, quote)
	if err != nil {
		return nil, err
	}
	if algo == "" {
		algo = "v1"
	}
	if err := shared.ValidatePartitionValue(algo); err != nil {
		return nil, err
	}
	path := filepath.Join(
		dataDir, "parquet", "ti",
		"provider="+provider,
		"base="+market.Base,
		"quote="+market.Quote,
		"algo="+algo,
		fmt.Sprintf("year=%d", year),
		"bars.parquet",
	)
	ctx := context.Background()
	ok, err := lakeobject.Exists(ctx, dataDir, path)
	if err != nil || !ok {
		return nil, err
	}
	f, size, err := lakeobject.Open(ctx, dataDir, path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	pf, err := parquet.OpenFile(f.File, size)
	if err != nil {
		return nil, err
	}
	reader := parquet.NewGenericReader[tiParquetRow](pf)
	defer reader.Close()
	var out []tiParquetRow
	buf := make([]tiParquetRow, 256)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			out = append(out, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	return out, nil
}

func indicatorID(method, typ string, hours int) (id, label string) {
	method = strings.TrimSpace(method)
	typ = strings.TrimSpace(typ)
	if method != "" && hours > 0 {
		id = method + "-" + strconv.Itoa(hours) + "h"
		label = strings.ToUpper(method) + " " + strconv.Itoa(hours) + "h"
		if typ != "" && typ != "MovingAverage" {
			// e.g. CloseDistance and MovingAverage both use method sma; the type keeps their ids apart.
			id = strings.ToLower(typ) + "-" + id
			label = typ + " " + label
		}
		return id, label
	}
	if typ != "" {
		return typ, typ
	}
	if method != "" {
		return method, method
	}
	return "", ""
}

// buildMarketSeries keeps candles whose open time (ms) falls in [fromSec, toSec].
// Indicator columns share that index; a missing sample is NaN.
func buildMarketSeries(candles []chartCandle, tiByOpen map[int64]map[string]float64, meta []chartIndicatorMeta, fromSec, toSec int64) *v1.ChartSeries {
	fromMs := fromSec * 1000
	toMs := toSec * 1000
	type row struct {
		time int64
		ohlc [4]float64
		ind  map[string]float64
	}
	rows := make([]row, 0, len(candles))
	for _, c := range candles {
		if c.OpenTime < fromMs || c.OpenTime > toMs {
			continue
		}
		rows = append(rows, row{time: c.OpenTime / 1000, ohlc: [4]float64{c.Open, c.High, c.Low, c.Close}, ind: tiByOpen[c.OpenTime]})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].time < rows[j].time })
	out := &v1.ChartSeries{
		TimeSec: make([]int64, len(rows)),
		Open:    make([]float64, len(rows)),
		High:    make([]float64, len(rows)),
		Low:     make([]float64, len(rows)),
		Close:   make([]float64, len(rows)),
	}
	columns := make([][]float64, len(meta))
	for i := range columns {
		columns[i] = make([]float64, len(rows))
	}
	for i, row := range rows {
		out.TimeSec[i] = row.time
		out.Open[i], out.High[i], out.Low[i], out.Close[i] = row.ohlc[0], row.ohlc[1], row.ohlc[2], row.ohlc[3]
		for n, m := range meta {
			v, ok := row.ind[m.ID]
			if !ok || math.IsNaN(v) {
				columns[n][i] = math.NaN()
				continue
			}
			columns[n][i] = v
		}
	}
	out.Indicators = make([]*v1.ChartIndicator, len(meta))
	for i, m := range meta {
		pane := v1.ChartPane_CHART_PANE_PRICE
		if m.Pane == "oscillator" {
			pane = v1.ChartPane_CHART_PANE_OSCILLATOR
		}
		style := v1.ChartStyle_CHART_STYLE_LINE
		if m.Style == "histogram" {
			style = v1.ChartStyle_CHART_STYLE_HISTOGRAM
		}
		out.Indicators[i] = &v1.ChartIndicator{
			Id: m.ID, Label: m.Label, Unit: m.Unit, Pane: pane, Style: style, Reference: m.Reference, Value: columns[i],
		}
	}
	return out
}

func marketSeries(ctx context.Context, exp experimentdb.Experiment, fromSec, toSec int64) (*v1.ChartSeries, error) {
	dataDir := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if dataDir == "" {
		dataDir = "/data"
	}
	provider := strings.TrimSpace(exp.Provider)
	if provider == "" {
		provider = "binance"
	}
	algo := strings.TrimSpace(exp.Algo)
	if algo == "" {
		algo = "v1"
	}
	key := exp.ID.String() + "|" + provider + "|" + exp.Base + "|" + exp.Quote + "|" + algo + "|" + strconv.FormatInt(fromSec, 10) + "|" + strconv.FormatInt(toSec, 10)
	marketSeriesCache.mu.Lock()
	if hit, ok := marketSeriesCache.m[key]; ok {
		marketSeriesCache.mu.Unlock()
		return proto.Clone(hit).(*v1.ChartSeries), nil
	}
	marketSeriesCache.mu.Unlock()

	candles, err := loadCandleWindow(ctx, dataDir, provider, exp.Base, exp.Quote, fromSec, toSec)
	if err != nil && !errors.Is(err, errChartParquetMissing) {
		return nil, err
	}
	tiByOpen, meta := loadIndicatorWindow(dataDir, provider, exp.Base, exp.Quote, algo, fromSec, toSec)
	series := buildMarketSeries(candles, tiByOpen, meta, fromSec, toSec)
	if len(series.TimeSec) > 0 {
		marketSeriesCache.mu.Lock()
		marketSeriesCache.m[key] = proto.Clone(series).(*v1.ChartSeries)
		marketSeriesCache.mu.Unlock()
	}
	return series, nil
}

func instanceChartSeries(inst experimentdb.ExperimentInstance, startCash float64) ([]*v1.ChartPoint, []*v1.ChartMarker) {
	db, err := openReadOnlySQLite(filepath.Join(instanceWorkspace(inst), "execution.sqlite"))
	if err != nil || db == nil {
		return nil, nil
	}
	defer db.Close()
	journals, _ := readJournalRows(db)
	rows, _ := readActivityRows(db)
	return equityFromJournals(journals, startCash), markersFromActivity(rows)
}

func equityFromJournals(journals []journalRow, startCash float64) []*v1.ChartPoint {
	type fill struct {
		closeMs int64
		side    string
		base    float64
		quote   float64
		price   float64
	}
	var fills []fill
	for _, j := range journals {
		ms := candleCloseFromLink(j.OrderLinkID)
		if ms == 0 {
			continue
		}
		base, err1 := strconv.ParseFloat(j.BaseQty, 64)
		quote, err2 := strconv.ParseFloat(j.QuoteQty, 64)
		price, err3 := strconv.ParseFloat(j.Price, 64)
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		fills = append(fills, fill{ms, strings.ToLower(j.Side), base, quote, price})
	}
	sort.Slice(fills, func(i, j int) bool { return fills[i].closeMs < fills[j].closeMs })
	if len(fills) == 0 {
		return nil
	}
	cash := startCash
	base := 0.0
	points := []*v1.ChartPoint{{TimeSec: fills[0].closeMs/1000 - 1, Value: startCash}}
	for _, f := range fills {
		switch f.side {
		case "buy":
			cash -= f.quote
			base += f.base
		case "sell":
			cash += f.quote
			base -= f.base
		}
		points = append(points, &v1.ChartPoint{TimeSec: f.closeMs / 1000, Value: cash + base*f.price})
	}
	return points
}

func candleCloseFromLink(link string) int64 {
	last := strings.LastIndex(link, ":")
	if last < 0 || last+1 >= len(link) {
		return 0
	}
	ms, err := strconv.ParseInt(link[last+1:], 10, 64)
	if err != nil || ms <= 0 {
		return 0
	}
	return ms
}

func markersFromActivity(rows []activityRow) []*v1.ChartMarker {
	var out []*v1.ChartMarker
	for _, row := range rows {
		if row.Kind != "fill" || row.CandleClose <= 0 {
			continue
		}
		raw := ""
		if row.Decision != nil {
			raw = strings.ToLower(*row.Decision)
		}
		if raw == "" && row.Side != nil {
			raw = strings.ToLower(*row.Side)
		}
		var side v1.ChartSide
		switch raw {
		case "buy":
			side = v1.ChartSide_CHART_SIDE_BUY
		case "sell":
			side = v1.ChartSide_CHART_SIDE_SELL
		default:
			continue
		}
		out = append(out, &v1.ChartMarker{TimeSec: row.CandleClose / 1000, Side: side})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TimeSec < out[j].TimeSec })
	return out
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(b)
}

// indicatorStyle is how the chart draws an indicator: MACD as a histogram, the rest as lines.
func indicatorStyle(typ string) string {
	if typ == "Macd" {
		return "histogram"
	}
	return "line"
}

// referenceLevels are horizontal guides for bounded oscillators.
func referenceLevels(typ string) []float64 {
	switch typ {
	case "Rsi":
		return []float64{30, 70}
	case "Macd":
		return []float64{0}
	}
	return nil
}

// indicatorPane puts oscillators in their own pane. A unit other than price is one signal. MACD and RSI are
// oscillators even though their stored unit is price (MACD) or index (RSI), so the type is checked too.
func indicatorPane(unit, typ string) string {
	if (unit != "" && unit != "price") || typ == "Macd" || typ == "Rsi" {
		return "oscillator"
	}
	return "price"
}
