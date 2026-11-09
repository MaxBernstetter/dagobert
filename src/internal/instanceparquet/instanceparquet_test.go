package instanceparquet

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/parquet-go/parquet-go"
)

var ctx = context.Background()

func strp(s string) *string   { return &s }
func f32p(f float32) *float32 { return &f }
func i32p(i int32) *int32     { return &i }

func order(link string, ordinal int32) Order {
	return Order{Ordinal: ordinal, OrderLinkID: link, Kind: "fill", Playbook: "ma", Symbol: "BTCUSDC", VenueSymbol: "BTCUSDC",
		CandleClose: int64(ordinal) + 1, Decision: strp("buy"), RetCode: i32p(0)}
}

// testShard has three instances: a full one, one with only a report and fills, and one with nothing but a report.
func testShard() []InstanceData {
	return []InstanceData{
		{
			InstanceID: "inst-a",
			ReportJSON: []byte(`{"version":1,"run":{}}`),
			Fills: []Fill{
				{TsMs: 1_700_000_000_000, Side: "Buy", BaseQty: 0.5, QuoteQty: 100.25, Price: 200.5, Fee: 0.1, FeeCurrency: "USDC"},
				{TsMs: 1_700_010_800_000, Side: "sell", BaseQty: 0.5, QuoteQty: 101, Price: 202, Fee: 0.2, FeeCurrency: "USDC"},
			},
			Orders: []Order{order("a1", 0), {Ordinal: 1, OrderLinkID: "a2", Kind: "no-trade", Playbook: "ma", Symbol: "BTCUSDC", VenueSymbol: "BTCUSDC", CandleClose: 2}},
			Journals: []Journal{{Ordinal: 0, OrderLinkID: "l", Venue: "paper", VenueSymbol: "BTCUSDC", Side: "Buy",
				BaseQty: "0.00100000", QuoteQty: "100.00000000", Price: "100000.00", Fee: "0.10", FeeCurrency: "USDC", PriceSource: "binance"}},
			Outcomes: []TradeOutcome{{Ordinal: 0, Side: 1, Quote: 12.5, Percent: f32p(0.03)}, {Ordinal: 1, Side: 0, Quote: -4}},
			Periods: []PeriodStat{
				{Kind: "month", Key: "2023-04", ReturnPct: func() *float64 { v := 0.02; return &v }(), TradeCount: i32p(3), WinCount: i32p(2)},
				{Kind: "year", Key: "2023", TradeCount: i32p(3)},
			},
		},
		{InstanceID: "inst-b", ReportJSON: []byte(`{"version":1,"run":{"b":true}}`), Fills: []Fill{{TsMs: 7, Side: "buy", Price: 9}}, Orders: []Order{order("b1", 0)}},
		{InstanceID: "inst-c", ReportJSON: []byte(`{"empty":true}`)},
	}
}

func writeTestShard(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "shards", "s1")
	if err := WriteShard(dir, "exp-1", testShard()); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRoundTripMultiInstanceShard(t *testing.T) {
	dir := writeTestShard(t)
	want := testShard()

	fills, err := ReadFills(ctx, dir, "", "inst-a")
	if err != nil {
		t.Fatal(err)
	}
	wantFills := want[0].Fills
	wantFills[0].Side = "buy" // sides are stored lowercase
	if !reflect.DeepEqual(fills, wantFills) {
		t.Fatalf("fills: got %+v want %+v", fills, wantFills)
	}
	orders, err := ReadOrders(ctx, dir, "", "inst-a")
	if err != nil || !reflect.DeepEqual(orders, want[0].Orders) {
		t.Fatalf("orders: %+v err=%v", orders, err)
	}
	if orders[1].Decision != nil || orders[1].RetCode != nil {
		t.Fatalf("optional fields must stay nil: %+v", orders[1])
	}
	journals, err := ReadJournals(ctx, dir, "", "inst-a")
	if err != nil || !reflect.DeepEqual(journals, want[0].Journals) {
		t.Fatalf("journals: %+v err=%v", journals, err)
	}
	outcomes, err := ReadTradeOutcomes(ctx, dir, "", "inst-a")
	if err != nil || !reflect.DeepEqual(outcomes, want[0].Outcomes) {
		t.Fatalf("outcomes: %+v err=%v", outcomes, err)
	}
	periods, err := ReadPeriodStats(ctx, dir, "", "inst-a")
	if err != nil || !reflect.DeepEqual(periods, want[0].Periods) {
		t.Fatalf("periods: %+v err=%v", periods, err)
	}
}

func TestReadReturnsOnlyTheRequestedInstance(t *testing.T) {
	dir := writeTestShard(t)
	orders, err := ReadOrders(ctx, dir, "", "inst-b")
	if err != nil || len(orders) != 1 || orders[0].OrderLinkID != "b1" {
		t.Fatalf("inst-b orders: %+v err=%v", orders, err)
	}
	fills, err := ReadFills(ctx, dir, "", "inst-b")
	if err != nil || len(fills) != 1 || fills[0].TsMs != 7 {
		t.Fatalf("inst-b fills: %+v err=%v", fills, err)
	}
	for id, want := range map[string]string{"inst-a": `{"version":1,"run":{}}`, "inst-b": `{"version":1,"run":{"b":true}}`, "inst-c": `{"empty":true}`} {
		got, err := ReadReport(ctx, dir, "", id)
		if err != nil || string(got) != want {
			t.Fatalf("report %s: %q err=%v", id, got, err)
		}
	}
}

func TestMissingInstanceAndMissingFileAreEmptyAndNotExist(t *testing.T) {
	dir := writeTestShard(t)
	rows, err := ReadOrders(ctx, dir, "", "inst-unknown")
	if !errors.Is(err, fs.ErrNotExist) || rows == nil || len(rows) != 0 {
		t.Fatalf("unknown instance: rows=%#v err=%v", rows, err)
	}
	if _, err := ReadReport(ctx, dir, "", "inst-unknown"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unknown report: %v", err)
	}
	rows, err = ReadOrders(ctx, dir, "nope", "inst-a")
	if !errors.Is(err, fs.ErrNotExist) || rows == nil || len(rows) != 0 {
		t.Fatalf("missing file: rows=%#v err=%v", rows, err)
	}
}

func TestInstanceWithoutRowsHasNoRowGroup(t *testing.T) {
	dir := writeTestShard(t)
	// inst-c has a report but no rows in any datatype: absent from those files, present in reports.
	for _, name := range []string{FillsFile, OrdersFile, JournalsFile, TradeOutcomesFile, PeriodStatsFile} {
		pf := openFooter(t, filepath.Join(dir, name))
		if _, ok := footerIndex(t, pf)["inst-c"]; ok {
			t.Fatalf("%s: empty instance must have no row group", name)
		}
	}
	if _, err := ReadFills(ctx, dir, "", "inst-c"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("empty instance fills: %v", err)
	}
	if _, err := ReadReport(ctx, dir, "", "inst-c"); err != nil {
		t.Fatalf("empty instance report: %v", err)
	}
}

func TestWriteShardWithNoRowsStillWritesReadableFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "s")
	if err := WriteShard(dir, "exp-1", []InstanceData{{InstanceID: "x", ReportJSON: []byte("{}")}}); err != nil {
		t.Fatal(err)
	}
	if rows, err := ReadFills(ctx, dir, "", "x"); !errors.Is(err, fs.ErrNotExist) || len(rows) != 0 {
		t.Fatalf("rows=%#v err=%v", rows, err)
	}
}

func openFooter(t *testing.T, path string) *parquet.File {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	info, _ := f.Stat()
	pf, err := parquet.OpenFile(f, info.Size())
	if err != nil {
		t.Fatal(err)
	}
	return pf
}

func footerIndex(t *testing.T, pf *parquet.File) map[string]int {
	t.Helper()
	raw, ok := pf.Lookup(KeyInstances)
	if !ok {
		t.Fatalf("footer %s missing", KeyInstances)
	}
	var idx map[string]int
	if err := json.Unmarshal([]byte(raw), &idx); err != nil {
		t.Fatal(err)
	}
	return idx
}

func TestFooterMapMatchesRowGroups(t *testing.T) {
	dir := writeTestShard(t)
	pf := openFooter(t, filepath.Join(dir, OrdersFile))
	idx := footerIndex(t, pf)
	if !reflect.DeepEqual(idx, map[string]int{"inst-a": 0, "inst-b": 1}) {
		t.Fatalf("index: %v", idx)
	}
	if n := len(pf.RowGroups()); n != 2 {
		t.Fatalf("row groups: %d", n)
	}
	if n := pf.RowGroups()[0].NumRows(); n != 2 {
		t.Fatalf("group 0 rows: %d", n)
	}
	reports := openFooter(t, filepath.Join(dir, ReportsFile))
	if n := len(reports.RowGroups()); n != 3 {
		t.Fatalf("reports row groups: %d", n)
	}
	for k, v := range map[string]string{KeySchemaVersion: "2", KeyExperimentID: "exp-1"} {
		if got, ok := pf.Lookup(k); !ok || got != v {
			t.Fatalf("footer %s: got %q ok=%v want %q", k, got, ok, v)
		}
	}
}

func TestWriteShardSkipsEmptySideFills(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "s")
	in := []InstanceData{{InstanceID: "x", ReportJSON: []byte("{}"), Fills: []Fill{{TsMs: 1, Side: "", Price: 1}, {TsMs: 2, Side: "Sell", Price: 2}}}}
	if err := WriteShard(dir, "e", in); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFills(ctx, dir, "", "x")
	if err != nil || len(got) != 1 || got[0].TsMs != 2 || got[0].Side != "sell" {
		t.Fatalf("fills: %+v err=%v", got, err)
	}
}

func TestWriteReplacesAtomicallyWithoutTempFiles(t *testing.T) {
	dir := t.TempDir()
	first := []InstanceData{{InstanceID: "x", ReportJSON: []byte("{}"), Orders: []Order{{OrderLinkID: "a", CandleClose: 1}}}}
	second := []InstanceData{{InstanceID: "x", ReportJSON: []byte("{}"), Orders: []Order{{OrderLinkID: "b", CandleClose: 2}, {OrderLinkID: "c", CandleClose: 3}}}}
	for _, in := range [][]InstanceData{first, second} {
		if err := WriteShard(dir, "e", in); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ReadOrders(ctx, dir, "", "x")
	if err != nil || len(got) != 2 {
		t.Fatalf("orders=%+v err=%v", got, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 6 {
		t.Fatalf("want exactly the six shard files, got %v", entries)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".parquet" {
			t.Fatalf("leftover file: %s", e.Name())
		}
	}
}

// subsetFill reads only two columns of fills.parquet.
type subsetFill struct {
	TsMs  int64   `parquet:"ts_ms"`
	Price float64 `parquet:"price"`
}

func TestReadProjectsSubsetOfColumns(t *testing.T) {
	dir := writeTestShard(t)
	pf := openFooter(t, filepath.Join(dir, FillsFile))
	r := parquet.NewGenericRowGroupReader[subsetFill](pf.RowGroups()[0])
	defer r.Close()
	got := make([]subsetFill, 2)
	if n, _ := r.Read(got); n != 2 || got[1] != (subsetFill{TsMs: 1_700_010_800_000, Price: 202}) {
		t.Fatalf("subset: n=%d %+v", n, got)
	}
}
