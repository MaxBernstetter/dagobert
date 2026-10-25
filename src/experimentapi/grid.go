package experimentapi

import (
	"fmt"
	"strconv"
	"strings"

	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
)

const (
	maxGridCombos   = 20_000
	gridSymbolKey   = "symbol"
	syncMaterialize = 250
)

var allowedSMA = map[int]bool{3: true, 6: true, 12: true, 24: true, 48: true}

func gridAxisValues(axis *v1.GridAxis) []interface{} {
	if axis == nil {
		return nil
	}
	out := make([]interface{}, 0, len(axis.Values)+len(axis.NumericValues))
	for _, n := range axis.NumericValues {
		out = append(out, n)
	}
	for _, s := range axis.Values {
		if n, err := strconv.ParseFloat(s, 64); err == nil {
			out = append(out, n)
			continue
		}
		out = append(out, s)
	}
	return out
}

// expandGrid — cartesian product of axis values (mirrors experiment-grid.ts).
func expandGrid(axes []*v1.GridAxis) []map[string]interface{} {
	if len(axes) == 0 {
		return []map[string]interface{}{}
	}
	combos := []map[string]interface{}{{}}
	for _, axis := range axes {
		vals := gridAxisValues(axis)
		if len(vals) == 0 {
			return []map[string]interface{}{}
		}
		var next []map[string]interface{}
		for _, base := range combos {
			for _, v := range vals {
				c := make(map[string]interface{}, len(base)+1)
				for k, val := range base {
					c[k] = val
				}
				c[axis.Key] = v
				next = append(next, c)
			}
		}
		combos = next
	}
	return combos
}

func isValidMaCrossConfig(cfg map[string]interface{}) bool {
	fastHr, ok1 := numberVal(cfg["fastHr"])
	slowHr, ok2 := numberVal(cfg["slowHr"])
	if !ok1 || !ok2 {
		return false
	}
	fi, si := int(fastHr), int(slowHr)
	if !allowedSMA[fi] || !allowedSMA[si] {
		return false
	}
	return fi < si
}

func isValidSignalConsensusConfig(cfg map[string]interface{}) bool {
	rsiBull, ok1 := numberVal(cfg["rsiBull"])
	rsiBear, ok2 := numberVal(cfg["rsiBear"])
	volRatioBull, ok3 := numberVal(cfg["volRatioBull"])
	volRatioBear, ok4 := numberVal(cfg["volRatioBear"])
	atrStopMult, ok5 := numberVal(cfg["atrStopMult"])
	minFlat, ok6 := numberVal(cfg["minBarsFlatAfterSell"])
	minLong, ok7 := numberVal(cfg["minBarsLongBeforeSell"])
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 || !ok7 {
		return false
	}
	if rsiBull <= rsiBear {
		return false
	}
	if volRatioBull <= volRatioBear || volRatioBull <= 0 || volRatioBear <= 0 {
		return false
	}
	if atrStopMult <= 0 {
		return false
	}
	if minFlat < 0 || minLong < 0 {
		return false
	}
	return true
}

func filterValidConfigs(configs []map[string]interface{}, playbook string) []map[string]interface{} {
	slug := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(playbook), ".yaml"), ".yml")
	var out []map[string]interface{}
	for _, c := range configs {
		if sym, ok := c[gridSymbolKey]; ok && strings.TrimSpace(fmt.Sprint(sym)) == "" {
			continue
		}
		switch slug {
		case "ma-cross":
			if isValidMaCrossConfig(c) {
				out = append(out, c)
			}
		case "signal-consensus":
			if isValidSignalConsensusConfig(c) {
				out = append(out, c)
			}
		default:
			out = append(out, c)
		}
	}
	return out
}

func splitGridCombo(cfg map[string]interface{}) (symbol string, strategy map[string]interface{}) {
	strategy = make(map[string]interface{})
	for k, v := range cfg {
		if k == gridSymbolKey {
			symbol = strings.TrimSpace(fmt.Sprint(v))
			continue
		}
		strategy[k] = v
	}
	return symbol, strategy
}

func specsFromGrid(playbook string, axes []*v1.GridAxis, fallbackSymbol string) ([]*v1.ExperimentInstanceSpec, error) {
	raw := expandGrid(axes)
	valid := filterValidConfigs(raw, playbook)
	if len(valid) == 0 {
		return nil, fmt.Errorf("No valid combinations (check constraints)")
	}
	if len(valid) > maxGridCombos {
		return nil, fmt.Errorf("Too many valid combinations (%d); max is %d", len(valid), maxGridCombos)
	}
	specs := make([]*v1.ExperimentInstanceSpec, 0, len(valid))
	for _, cfg := range valid {
		sym, strategyConfig := splitGridCombo(cfg)
		s := strings.TrimSpace(sym)
		if s == "" {
			s = fallbackSymbol
		}
		spec := &v1.ExperimentInstanceSpec{
			Playbook: playbook,
			Symbol:   strings.ToUpper(s),
		}
		switch normalizePlaybook(playbook) {
		case "ma-cross":
			mc, err := normalizeMaCrossConfig(strategyConfig)
			if err != nil {
				return nil, err
			}
			spec.MaCross = mc
		case "signal-consensus":
			sc, err := normalizeSignalConsensusConfig(strategyConfig)
			if err != nil {
				return nil, err
			}
			spec.SignalConsensus = sc
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

func numberVal(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	default:
		if v == nil {
			return 0, false
		}
		return 0, false
	}
}
