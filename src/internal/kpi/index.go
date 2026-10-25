package kpi

import (
	"encoding/json"
	"sync"

	"github.com/MaxBernstetter/dagobert/src/internal/canonical"
)

var defKeyCache sync.Map // string(defJSON) -> canonical key string

func cachedDefKey(raw json.RawMessage) (string, bool) {
	s := string(raw)
	if v, ok := defKeyCache.Load(s); ok {
		return v.(string), true
	}
	got, err := canonical.ParseDefFromJSON(raw)
	if err != nil {
		return "", false
	}
	key, err := got.String()
	if err != nil {
		return "", false
	}
	defKeyCache.Store(s, key)
	return key, true
}

// IndexResults builds canonical-def-key → float for one bar's results.
func IndexResults(results []Result) map[string]float64 {
	if len(results) == 0 {
		return nil
	}
	out := make(map[string]float64, len(results))
	for _, r := range results {
		key, ok := cachedDefKey(r.Def)
		if !ok {
			continue
		}
		if f, ok := jsonNumberToFloat(r.Value); ok {
			out[key] = f
		}
	}
	return out
}

// Feature looks up an indicator by canonical key from an index map.
func Feature(index map[string]float64, typ, method string, timeFrameHr int) (float64, bool) {
	if index == nil {
		return 0, false
	}
	def := canonical.IndicatorDef{Type: typ, Method: method, TimeFrameHr: timeFrameHr}
	key, err := def.String()
	if err != nil {
		return 0, false
	}
	v, ok := index[key]
	return v, ok
}
