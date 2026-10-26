package candlefile

import (
	"bytes"
	"errors"
	"strconv"
	"testing"

	"github.com/parquet-go/parquet-go"

	"github.com/MaxBernstetter/dagobert/src/internal/parquetmeta"
)

type legacyRow struct {
	OpenTime  int64  `parquet:"open_time"`
	Open      string `parquet:"open"`
	High      string `parquet:"high"`
	Low       string `parquet:"low"`
	Close     string `parquet:"close"`
	Volume    string `parquet:"volume"`
	CloseTime int64  `parquet:"close_time"`
}

func open(t *testing.T, b []byte) *parquet.File {
	t.Helper()
	pf, err := parquet.OpenFile(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	return pf
}

func TestDecimalRoundTripAndFloat(t *testing.T) {
	rows := []Row{
		{1, "82409.43000000", "1", "0.00000001", "-0.5", "1069.70305000", 2},
		{3, "100.25", "99999999.99999999", "0", "12", "12345678901234567890.12345678", 4},
		{5, "7", "7", "7", "7", "99999999999999999999.99999999", 6},
	}
	var buf bytes.Buffer
	if err := Write(&buf, rows, parquetmeta.Meta{IntervalMS: 3600000}); err != nil {
		t.Fatal(err)
	}
	pf := open(t, buf.Bytes())
	if IsLegacy(pf) {
		t.Fatal("new file reported legacy")
	}
	got, err := Read(pf)
	if err != nil {
		t.Fatal(err)
	}
	fl, err := ReadFloat(pf)
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range rows {
		want := [5]string{r.Open, r.High, r.Low, r.Close, r.Volume}
		have := [5]string{got[i].Open, got[i].High, got[i].Low, got[i].Close, got[i].Volume}
		wf := [5]float64{fl[i].Open, fl[i].High, fl[i].Low, fl[i].Close, fl[i].Volume}
		for c := range want {
			if NormalizeDecimal(want[c]) != NormalizeDecimal(have[c]) {
				t.Errorf("row %d col %d: got %s want %s", i, c, have[c], want[c])
			}
			f, _ := strconv.ParseFloat(want[c], 64)
			if wf[c] != f {
				t.Errorf("row %d col %d float: got %v want %v", i, c, wf[c], f)
			}
		}
	}
}

func TestLegacyFileReadsTheSame(t *testing.T) {
	var buf bytes.Buffer
	old := []legacyRow{{1, "100.25", "101.5", "99.75", "100.5", "12.345", 2}}
	if err := parquet.Write(&buf, old); err != nil {
		t.Fatal(err)
	}
	pf := open(t, buf.Bytes())
	if !IsLegacy(pf) {
		t.Fatal("legacy file not detected")
	}
	rows, err := Read(pf)
	if err != nil || len(rows) != 1 || rows[0].Close != "100.5" {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	fl, err := ReadFloat(pf)
	if err != nil || fl[0].Volume != 12.345 {
		t.Fatalf("floats=%v err=%v", fl, err)
	}
}

func TestWriteRejectsInexactValues(t *testing.T) {
	cases := map[string]Row{
		"decimals": {Open: "1.123456789", High: "1", Low: "1", Close: "1", Volume: "1"},
		"price":    {Open: "10000000000.00000000", High: "1", Low: "1", Close: "1", Volume: "1"},
		"volume":   {Open: "1", High: "1", Low: "1", Close: "1", Volume: "100000000000000000000"},
		"junk":     {Open: "1e5", High: "1", Low: "1", Close: "1", Volume: "1"},
		"empty":    {Open: "1", High: "", Low: "1", Close: "1", Volume: "1"},
	}
	for name, r := range cases {
		var buf bytes.Buffer
		err := Write(&buf, []Row{r}, parquetmeta.Meta{})
		var ve *ValueError
		if !errors.As(err, &ve) {
			t.Errorf("%s: want ValueError, got %v", name, err)
		}
		if buf.Len() != 0 {
			t.Errorf("%s: wrote bytes before failing", name)
		}
	}
	// Trailing zeros past scale 8 are exact.
	if _, err := EncodePrice("open", "1.230000000"); err != nil {
		t.Errorf("trailing zeros rejected: %v", err)
	}
}
