package experiment_test

import (
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/parquet-go/parquet-go"
)

// The experiment instance files are read with parquet-go directly, as the API does not
// belong to the test: fills.parquet and journals.parquet of one instance workspace.

type journalFile struct {
	OrderLinkID string `parquet:"order_link_id"`
	Side        string `parquet:"side"`
	BaseQty     string `parquet:"base_qty"`
	QuoteQty    string `parquet:"quote_qty"`
	Price       string `parquet:"price"`
}

// readJournalFile returns the journal rows of one instance; a missing file is no rows.
func readJournalFile(t *testing.T, ws string) []journalFile {
	t.Helper()
	rows, err := parquet.ReadFile[journalFile](filepath.Join(ws, "journals.parquet"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

type outcomeFile struct {
	Ordinal int32   `parquet:"ordinal"`
	Side    int32   `parquet:"side"`
	Quote   float32 `parquet:"quote"`
}

// readOutcomeFile returns the trade outcome rows of one instance; a missing file is no rows.
func readOutcomeFile(t *testing.T, ws string) []outcomeFile {
	t.Helper()
	rows, err := parquet.ReadFile[outcomeFile](filepath.Join(ws, "trade_outcomes.parquet"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

type fillFile struct {
	TsMs     int64   `parquet:"ts_ms"`
	Side     string  `parquet:"side"`
	QuoteQty float64 `parquet:"quote_qty"`
}

func readFillFile(t *testing.T, ws string) []fillFile {
	t.Helper()
	rows, err := parquet.ReadFile[fillFile](filepath.Join(ws, "fills.parquet"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

// decisionCloses lists the candle close of each fill on the given side ("buy" or "sell"), read from
// the order link id, which is playbookKey:symbol:candleClose.
func decisionCloses(t *testing.T, workspace, decision string) []int64 {
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

func countSignalsWithDecision(t *testing.T, workspace, decision string) int {
	return len(decisionCloses(t, workspace, decision))
}

func countFills(t *testing.T, workspace string) int {
	t.Helper()
	return len(readJournalFile(t, workspace))
}

// distinctPlaybooks lists the playbook keys of the fills, from the order link ids.
func distinctPlaybooks(t *testing.T, workspace string) []string {
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
func buyQuotes(t *testing.T, ws string) []float64 {
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
func firstBuyQuote(t *testing.T, ws string) (float64, bool) {
	t.Helper()
	q := buyQuotes(t, ws)
	if len(q) == 0 {
		return 0, false
	}
	return q[0], true
}

// netCashFlow sums the quote of sells less buys over the fills (finite for any valid fill set).
func netCashFlow(t *testing.T, ws string) float64 {
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
