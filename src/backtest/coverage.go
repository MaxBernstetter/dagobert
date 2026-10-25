package backtest

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/parquet-go/parquet-go"
	_ "modernc.org/sqlite"

	"github.com/MaxBernstetter/dagobert/src/internal/instruments"
	"github.com/MaxBernstetter/dagobert/src/internal/lakeobject"
	"github.com/MaxBernstetter/dagobert/src/shared"
)

const hourMS = int64(3_600_000)

type hourCandleRow struct {
	OpenTime int64 `parquet:"open_time"`
}

// CoverageReport describes lake coverage for closed hours in [from, to).
type CoverageReport struct {
	Provider         string
	Base             string
	Quote            string
	FromMs           int64
	ToMs             int64
	Expected         int
	Have             int
	MissingHours     []int64
	UnavailableHours []int64
	Ready            bool
}

// CheckRawCoverage lists closed hour opens in [from, to) missing from parquet.
// provider may be empty; then instruments.PriceProvider for the pipeline symbol is used.
func CheckRawCoverage(dataDir, provider, base, quote string, fromMs, toMs int64) (CoverageReport, error) {
	m, err := shared.NormalizeMarket(base, quote)
	if err != nil {
		return CoverageReport{}, err
	}
	if provider == "" {
		provider = instruments.PriceProvider(shared.PipelineSymbol("", m.Base, m.Quote))
	}
	rep := CoverageReport{Provider: provider, Base: m.Base, Quote: m.Quote, FromMs: fromMs, ToMs: toMs}
	expected, ok := hourOpens(fromMs, toMs)
	if !ok {
		return rep, fmt.Errorf("hour grid overflow")
	}
	now := time.Now().UnixMilli()
	var closed []int64
	for _, open := range expected {
		if stillOpenHour(open, now) {
			continue
		}
		closed = append(closed, open)
	}
	rep.Expected = len(closed)

	paths, err := lakeobject.ListCandleParquetAbsPathsInRange(context.Background(), dataDir, provider, m.Base, m.Quote, fromMs, toMs)
	if err != nil {
		return rep, err
	}
	have, err := parquetOpenTimesMany(dataDir, paths)
	if err != nil {
		return rep, err
	}
	unavail, err := listUnavailableHours(dataDir, provider, m.Base, m.Quote, fromMs, toMs)
	if err != nil {
		return rep, err
	}
	unavailSet := make(map[int64]struct{}, len(unavail))
	for _, t := range unavail {
		unavailSet[t] = struct{}{}
	}

	missing := make([]int64, 0)
	unavailableInRange := make([]int64, 0)
	haveCount := 0
	for _, open := range closed {
		if _, ok := have[open]; ok {
			haveCount++
			continue
		}
		if _, ok := unavailSet[open]; ok {
			unavailableInRange = append(unavailableInRange, open)
			continue
		}
		missing = append(missing, open)
	}
	rep.Have = haveCount
	rep.MissingHours = missing
	rep.UnavailableHours = unavailableInRange
	rep.Ready = len(missing) == 0 && rep.Expected > 0
	if rep.Expected == 0 {
		rep.Ready = false
	}
	return rep, nil
}

func stillOpenHour(open, now int64) bool {
	if open > math.MaxInt64-hourMS+1 {
		return true
	}
	return open+hourMS-1 > now
}

func hourOpens(from, to int64) ([]int64, bool) {
	t := from
	if rem := t % hourMS; rem != 0 {
		if t >= 0 {
			delta := hourMS - rem
			if t > math.MaxInt64-delta {
				return nil, false
			}
			t += delta
		} else {
			t -= rem
		}
	}
	if t < from {
		return nil, false
	}
	var out []int64
	for t < to {
		out = append(out, t)
		if t > math.MaxInt64-hourMS {
			break
		}
		t += hourMS
	}
	return out, true
}

func parquetOpenTimesMany(dataDir string, paths []string) (map[int64]struct{}, error) {
	out := map[int64]struct{}{}
	for _, path := range paths {
		part, err := parquetOpenTimes(dataDir, path)
		if err != nil {
			return nil, err
		}
		for t := range part {
			out[t] = struct{}{}
		}
	}
	return out, nil
}

func parquetOpenTimes(dataDir, path string) (map[int64]struct{}, error) {
	out := map[int64]struct{}{}
	ctx := context.Background()
	ok, err := lakeobject.Exists(ctx, dataDir, path)
	if err != nil {
		return nil, err
	}
	if !ok {
		return out, nil
	}
	f, size, err := lakeobject.Open(ctx, dataDir, path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows, err := parquet.Read[hourCandleRow](f.File, size)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.OpenTime] = struct{}{}
	}
	return out, nil
}

func listUnavailableHours(dataDir, provider, base, quote string, fromMs, toMs int64) ([]int64, error) {
	path := filepath.Join(dataDir, "ingest.sqlite")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs), RawQuery: "_pragma=busy_timeout(5000)"}
	db, err := sql.Open("sqlite", u.String()+"&mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	hasBase, err := columnExists(db, "unavailable", "base")
	if err != nil {
		return nil, err
	}
	var rows *sql.Rows
	if hasBase {
		rows, err = db.Query(
			`SELECT open_time FROM unavailable WHERE provider = ? AND base = ? AND quote = ? AND open_time >= ? AND open_time < ? ORDER BY open_time`,
			provider, base, quote, fromMs, toMs,
		)
	} else {
		// Pre-cutover DB: ignore (operators wipe).
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var t int64
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func columnExists(db *sql.DB, table, col string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == col {
			return true, nil
		}
	}
	return false, rows.Err()
}

func formatCoverageGapSample(hours []int64, n int) string {
	if len(hours) == 0 || n <= 0 {
		return ""
	}
	if n > len(hours) {
		n = len(hours)
	}
	s := formatUTC(hours[0])
	for i := 1; i < n; i++ {
		s += ", " + formatUTC(hours[i])
	}
	if len(hours) > n {
		s += ", …"
	}
	return s
}
