package store

import (
	"testing"
)

func TestOpenTwiceWithFillBackfill(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	body := map[string]string{
		"orderLinkId":  "ma-cross.yaml:BTCUSDT:1700000000000",
		"venueSymbol":  "BTCUSDC",
		"side":         "Buy",
		"cumExecQty":   "0.001",
		"cumExecValue": "5",
		"avgPrice":     "50000",
		"orderId":      "oid-1",
	}
	if err := st.InsertOrderResult(body["orderLinkId"], KindFill, body); err != nil {
		t.Fatal(err)
	}
	st.Close()

	st2, err := Open(dir)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	st2.Close()

	st3, err := Open(dir)
	if err != nil {
		t.Fatalf("third open: %v", err)
	}
	st3.Close()
}
