package experiment_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/parquet-go/parquet-go"
)

// The shard files are read with parquet-go directly, as the API does not belong to the test: the
// footer key trading_copilot.instances maps an instance id to its row group, and only that row group
// is read.

// instRef locates one instance's rows: its shard directory under dataDir and its id. An empty dir is
// an instance without shard_key (failed, not run).
type instRef struct {
	dir, id string
}

// instanceRef is the shard reference of one instance, from the shard_key in the catalog.
func instanceRef(dataDir string, inst pgInst) instRef {
	if inst.ShardKey == "" {
		return instRef{id: inst.ID}
	}
	return instRef{dir: filepath.Join(dataDir, inst.ShardKey), id: inst.ID}
}

// readRows returns the rows of one instance in one shard file; no shard, no file or no row group is no rows.
func readRows[T any](t *testing.T, ref instRef, file string) []T {
	t.Helper()
	if ref.dir == "" {
		return nil
	}
	f, err := os.Open(filepath.Join(ref.dir, file))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	pf, err := parquet.OpenFile(f, info.Size())
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := pf.Lookup("trading_copilot.instances")
	if !ok {
		t.Fatalf("%s: footer trading_copilot.instances missing", file)
	}
	var index map[string]int
	if err := json.Unmarshal([]byte(raw), &index); err != nil {
		t.Fatal(err)
	}
	g, ok := index[ref.id]
	if !ok {
		return nil
	}
	rg := pf.RowGroups()[g]
	rows := make([]T, rg.NumRows())
	r := parquet.NewGenericRowGroupReader[T](rg)
	defer r.Close()
	if n, err := r.Read(rows); int64(n) != rg.NumRows() {
		t.Fatalf("%s: read %d of %d rows: %v", file, n, rg.NumRows(), err)
	}
	return rows
}

type reportFile struct {
	InstanceID string `parquet:"instance_id"`
	ReportJSON string `parquet:"report_json"`
}

// readReport returns the report JSON of one instance from reports.parquet; it fails when there is none.
func readReport(t *testing.T, ref instRef) []byte {
	t.Helper()
	rows := readRows[reportFile](t, ref, "reports.parquet")
	if len(rows) != 1 || rows[0].InstanceID != ref.id {
		t.Fatalf("instance %s: want one report row in %s, got %+v", ref.id, ref.dir, rows)
	}
	return []byte(rows[0].ReportJSON)
}

type journalFile struct {
	OrderLinkID string `parquet:"order_link_id"`
	Side        string `parquet:"side"`
	BaseQty     string `parquet:"base_qty"`
	QuoteQty    string `parquet:"quote_qty"`
	Price       string `parquet:"price"`
}

// readJournalFile returns the journal rows of one instance; a missing file is no rows.
func readJournalFile(t *testing.T, ref instRef) []journalFile {
	t.Helper()
	return readRows[journalFile](t, ref, "journals.parquet")
}

type outcomeFile struct {
	Ordinal int32   `parquet:"ordinal"`
	Side    int32   `parquet:"side"`
	Quote   float32 `parquet:"quote"`
}

// readOutcomeFile returns the trade outcome rows of one instance; a missing file is no rows.
func readOutcomeFile(t *testing.T, ref instRef) []outcomeFile {
	t.Helper()
	return readRows[outcomeFile](t, ref, "trade_outcomes.parquet")
}

type fillFile struct {
	TsMs     int64   `parquet:"ts_ms"`
	Side     string  `parquet:"side"`
	QuoteQty float64 `parquet:"quote_qty"`
}

func readFillFile(t *testing.T, ref instRef) []fillFile {
	t.Helper()
	return readRows[fillFile](t, ref, "fills.parquet")
}

// decisionCloses lists the candle close of each fill on the given side ("buy" or "sell"), read from
// the order link id, which is playbookKey:symbol:candleClose.
func decisionCloses(t *testing.T, workspace instRef, decision string) []int64 {
	t.Helper()
	side := map[string]string{"buy": "Buy", "sell": "Sell"}[decision]
	if side == "" {
		return nil
	}
	var out []int64
	for _, j := range readJournalFile(t, workspace) {
		if j.Side != side {
			continue
		}
		last := strings.LastIndex(j.OrderLinkID, ":")
		if last <= 0 {
			continue
		}
		ms, err := strconv.ParseInt(j.OrderLinkID[last+1:], 10, 64)
		if err != nil || ms <= 0 {
			continue
		}
		out = append(out, ms)
	}
	return out
}

func countSignalsWithDecision(t *testing.T, workspace instRef, decision string) int {
	return len(decisionCloses(t, workspace, decision))
}

func countFills(t *testing.T, workspace instRef) int {
	t.Helper()
	return len(readJournalFile(t, workspace))
}

// distinctPlaybooks lists the playbook keys of the fills, from the order link ids.
func distinctPlaybooks(t *testing.T, workspace instRef) []string {
	t.Helper()
	seen := map[string]struct{}{}
	var out []string
	for _, j := range readJournalFile(t, workspace) {
		last := strings.LastIndex(j.OrderLinkID, ":")
		if last <= 0 {
			continue
		}
		rest := j.OrderLinkID[:last]
		prev := strings.LastIndex(rest, ":")
		if prev <= 0 {
			continue
		}
		key := rest[:prev]
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// buyQuotes lists the quote spent by each buy fill, ordered by order link id.
func buyQuotes(t *testing.T, ws instRef) []float64 {
	t.Helper()
	var links []string
	quotes := map[string]float64{}
	for _, j := range readJournalFile(t, ws) {
		if j.Side != "Buy" {
			continue
		}
		q, err := strconv.ParseFloat(j.QuoteQty, 64)
		if err != nil {
			t.Fatal(err)
		}
		links = append(links, j.OrderLinkID)
		quotes[j.OrderLinkID] = q
	}
	sort.Strings(links)
	out := make([]float64, 0, len(links))
	for _, l := range links {
		out = append(out, quotes[l])
	}
	return out
}

// firstBuyQuote returns the quote spent by the first buy in order link id order.
func firstBuyQuote(t *testing.T, ws instRef) (float64, bool) {
	t.Helper()
	q := buyQuotes(t, ws)
	if len(q) == 0 {
		return 0, false
	}
	return q[0], true
}

// netCashFlow sums the quote of sells less buys over the fills (finite for any valid fill set).
func netCashFlow(t *testing.T, ws instRef) float64 {
	t.Helper()
	var net float64
	for _, f := range readFillFile(t, ws) {
		if f.Side == "sell" {
			net += f.QuoteQty
		} else {
			net -= f.QuoteQty
		}
	}
	return net
}
