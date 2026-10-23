package shared

import (
	"encoding/json"
	"testing"
)

func TestDataSourceJSON(t *testing.T) {
	raw, err := json.Marshal(DataSourceBinance)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `"binance"` {
		t.Fatalf("marshal: %s", raw)
	}
	var back DataSource
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back != DataSourceBinance {
		t.Fatalf("unmarshal: %v", back)
	}
}
