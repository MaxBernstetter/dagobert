package instanceparquet

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/parquet-go/parquet-go"
)

var testMeta = Meta{ExperimentID: "exp-1", InstanceID: "inst-1", PlaybookKey: "ma-cross:abc", Symbol: "BTCUSDC"}

func strp(s string) *string   { return &s }
func f32p(f float32) *float32 { return &f }
func i32p(i int32) *int32     { return &i }

func TestRoundTripFills(t *testing.T) {
	dir := t.TempDir()
	in := []Fill{
		{TsMs: 1_700_000_000_000, Side: "buy", BaseQty: 0.5, QuoteQty: 100.25, Price: 200.5, Fee: 0.1, FeeCurrency: "USDC"},
		{TsMs: 1_700_010_800_000, Side: "sell", BaseQty: 0.5, QuoteQty: 101, Price: 202, Fee: 0.2, FeeCurrency: "USDC"},
	}
	if err := WriteFills(dir, testMeta, in); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFills(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("fills: got %+v want %+v", got, in)
	}
}

func TestWriteFillsSkipsEmptySideAndLowercases(t *testing.T) {
	dir := t.TempDir()
	in := []Fill{
		{TsMs: 1, Side: "", Price: 1},
		{TsMs: 2, Side: "Sell", Price: 2},
	}
	if err := WriteFills(dir, testMeta, in); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFills(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].TsMs != 2 || got[0].Side != "sell" {
		t.Fatalf("fills: %+v", got)
	}
}

func TestRoundTripOrdersKeepsNulls(t *testing.T) {
	dir := t.TempDir()
	in := []Order{
		{Ordinal: 0, OrderLinkID: "ma:BTC:1", Kind: "fill", Playbook: "ma", Symbol: "BTCUSDC", VenueSymbol: "BTCUSDC",
			CandleClose: 1, Decision: strp("buy"), OrderID: strp("o-1"), OrderStatus: strp("Filled"), Side: strp("Buy"),
			Qty: strp("0.5"), CumExecQty: strp("0.5"), CumExecValue: strp("100.25"), AvgPrice: strp("200.5"),
			CumExecFee: strp("0.1"), FeeCurrency: strp("USDC"), MarketUnit: strp("baseCoin"), RetCode: i32p(0), RetMsg: strp("OK")},
		{Ordinal: 1, OrderLinkID: "ma:BTC:2", Kind: "no-trade", Playbook: "ma", Symbol: "BTCUSDC", VenueSymbol: "BTCUSDC", CandleClose: 2},
	}
	if err := WriteOrders(dir, testMeta, in); err != nil {
		t.Fatal(err)
	}
	got, err := ReadOrders(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("orders: got %+v want %+v", got, in)
	}
	if got[1].Decision != nil || got[1].RetCode != nil {
		t.Fatalf("optional fields must stay nil: %+v", got[1])
	}
}

func TestRoundTripJournalsAndOutcomes(t *testing.T) {
	dir := t.TempDir()
	journals := []Journal{{Ordinal: 0, OrderLinkID: "l", Venue: "paper", VenueSymbol: "BTCUSDC", Side: "Buy",
		BaseQty: "0.00100000", QuoteQty: "100.00000000", Price: "100000.00", Fee: "0.10", FeeCurrency: "USDC",
		ExchangeOrderID: "", PriceSource: "binance"}}
	if err := WriteJournals(dir, testMeta, journals); err != nil {
		t.Fatal(err)
	}
	gotJ, err := ReadJournals(dir)
	if err != nil || !reflect.DeepEqual(gotJ, journals) {
		t.Fatalf("journals: %+v err=%v", gotJ, err)
	}

	stats := []PeriodStat{
		{Kind: "month", Key: "2023-04", ReturnPct: func() *float64 { v := 0.02; return &v }(), TradeCount: i32p(3), WinCount: i32p(2)},
		{Kind: "year", Key: "2023", TradeCount: i32p(3)},
	}
	if err := WritePeriodStats(dir, testMeta, stats); err != nil {
		t.Fatal(err)
	}
	gotS, err := ReadPeriodStats(dir)
	if err != nil || !reflect.DeepEqual(gotS, stats) {
		t.Fatalf("period stats: %+v err=%v", gotS, err)
	}

	outcomes := []TradeOutcome{{Ordinal: 0, Side: 1, Quote: 12.5, Percent: f32p(0.03)}, {Ordinal: 1, Side: 0, Quote: -4}}
	if err := WriteTradeOutcomes(dir, testMeta, outcomes); err != nil {
		t.Fatal(err)
	}
	gotO, err := ReadTradeOutcomes(dir)
	if err != nil || !reflect.DeepEqual(gotO, outcomes) {
		t.Fatalf("outcomes: %+v err=%v", gotO, err)
	}
}

func TestMissingFileIsEmptyAndNotExist(t *testing.T) {
	dir := t.TempDir()
	rows, err := ReadOrders(dir)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err: %v", err)
	}
	if rows == nil || len(rows) != 0 {
		t.Fatalf("rows: %#v", rows)
	}
}

func TestEmptyRowsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := WriteFills(dir, testMeta, nil); err != nil {
		t.Fatal(err)
	}
	rows, err := ReadFills(dir)
	if err != nil || rows == nil || len(rows) != 0 {
		t.Fatalf("rows=%#v err=%v", rows, err)
	}
}

func TestFooterAndSingleRowGroup(t *testing.T) {
	dir := t.TempDir()
	rows := make([]Fill, 500)
	for i := range rows {
		rows[i] = Fill{TsMs: int64(i), Side: "buy", Price: 1}
	}
	if err := WriteFills(dir, testMeta, rows); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(dir, FillsFile))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	info, _ := f.Stat()
	pf, err := parquet.OpenFile(f, info.Size())
	if err != nil {
		t.Fatal(err)
	}
	if n := len(pf.RowGroups()); n != 1 {
		t.Fatalf("row groups: %d", n)
	}
	want := map[string]string{
		KeySchemaVersion: "1", KeyExperimentID: "exp-1", KeyInstanceID: "inst-1",
		KeyPlaybookKey: "ma-cross:abc", KeySymbol: "BTCUSDC",
	}
	for k, v := range want {
		if got, ok := pf.Lookup(k); !ok || got != v {
			t.Fatalf("footer %s: got %q ok=%v want %q", k, got, ok, v)
		}
	}
}

// subsetFill reads only two columns of fills.parquet.
type subsetFill struct {
	TsMs  int64   `parquet:"ts_ms"`
	Price float64 `parquet:"price"`
}

func TestReadProjectsSubsetOfColumns(t *testing.T) {
	dir := t.TempDir()
	in := []Fill{{TsMs: 5, Side: "buy", BaseQty: 1, QuoteQty: 2, Price: 2, Fee: 0.1, FeeCurrency: "USDC"}}
	if err := WriteFills(dir, testMeta, in); err != nil {
		t.Fatal(err)
	}
	got, err := parquet.ReadFile[subsetFill](filepath.Join(dir, FillsFile))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != (subsetFill{TsMs: 5, Price: 2}) {
		t.Fatalf("subset: %+v", got)
	}
}

func TestWriteReplacesAtomicallyWithoutTempFiles(t *testing.T) {
	dir := t.TempDir()
	if err := WriteOrders(dir, testMeta, []Order{{OrderLinkID: "a", CandleClose: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := WriteOrders(dir, testMeta, []Order{{OrderLinkID: "b", CandleClose: 2}, {OrderLinkID: "c", CandleClose: 3}}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadOrders(dir)
	if err != nil || len(got) != 2 {
		t.Fatalf("orders=%+v err=%v", got, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("leftover files: %v", entries)
	}
}

func TestRemoveDeletesAllFilesAndIgnoresMissing(t *testing.T) {
	dir := t.TempDir()
	if err := WriteFills(dir, testMeta, []Fill{{Side: "buy"}}); err != nil {
		t.Fatal(err)
	}
	if err := WriteJournals(dir, testMeta, []Journal{{OrderLinkID: "x"}}); err != nil {
		t.Fatal(err)
	}
	if err := WritePeriodStats(dir, testMeta, []PeriodStat{{Kind: "year", Key: "2023"}}); err != nil {
		t.Fatal(err)
	}
	if err := Remove(dir); err != nil {
		t.Fatal(err)
	}
	if err := Remove(dir); err != nil {
		t.Fatalf("second remove: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("files left: %v", entries)
	}
}
