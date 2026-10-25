package experimentapi

import "net/http"

// experimentReadyStrategies lists the playbooks that accept a strategyConfig and can be replayed
// by the experiment runner, with the options the create form renders for each. It is an allowlist:
// add a playbook here only when the runner can validate and replay its config.
var experimentReadyStrategies = []strategyView{
	{
		ID:              "ma-cross",
		Label:           "Moving-Average Cross",
		ExperimentReady: true,
		Options: []strategyOption{
			enumOption("fastHr", "Fast window", "Shorter SMA lookback in calendar hours (must divide bar duration).", 3),
			enumOption("slowHr", "Slow window", "Longer SMA lookback; must be greater than the fast window.", 6),
		},
	},
	{
		ID:              "signal-consensus",
		Label:           "Signal consensus",
		ExperimentReady: true,
		Options: []strategyOption{
			numberOption("rsiBull", "RSI bull", "RSI at or above this counts as bullish momentum.", 50, 80, 1, "", 52),
			numberOption("rsiBear", "RSI bear", "RSI at or below this counts as bearish momentum.", 20, 50, 1, "", 48),
			numberOption("volRatioBull", "Volume bull ratio", "Bar volume / mean at or above this is bullish.", 1, 3, 0.05, "", 1.1),
			numberOption("volRatioBear", "Volume bear ratio", "Bar volume / mean at or below this is bearish.", 0.2, 1, 0.05, "", 0.85),
			numberOption("atrStopMult", "ATR stop mult", "Sell when close < SMA12 − mult × ATR24.", 0.5, 5, 0.25, "×", 2.5),
			numberOption("minBarsFlatAfterSell", "Flat cooldown", "Min 3h bars flat after a sell before a new buy.", 0, 48, 1, "bars", 8),
			numberOption("minBarsLongBeforeSell", "Long cooldown", "Min 3h bars in a long before a sell is allowed.", 0, 48, 1, "bars", 4),
		},
	},
}

var smaHourValues = []strategyValue{
	{Value: 3, Label: "3h"},
	{Value: 6, Label: "6h"},
	{Value: 12, Label: "12h"},
	{Value: 24, Label: "24h"},
	{Value: 48, Label: "48h"},
}

type strategyView struct {
	ID              string           `json:"id"`
	Label           string           `json:"label"`
	Description     string           `json:"description,omitempty"`
	ExperimentReady bool             `json:"experimentReady"`
	Options         []strategyOption `json:"options"`
}

// strategyOption is either an enum (Values set) or a number (Min and Max set).
type strategyOption struct {
	Key         string          `json:"key"`
	Label       string          `json:"label"`
	Description string          `json:"description,omitempty"`
	Kind        string          `json:"kind"`
	ValueType   string          `json:"valueType,omitempty"`
	Values      []strategyValue `json:"values,omitempty"`
	Min         *float64        `json:"min,omitempty"`
	Max         *float64        `json:"max,omitempty"`
	Step        *float64        `json:"step,omitempty"`
	Unit        string          `json:"unit,omitempty"`
	Default     any             `json:"default"`
}

type strategyValue struct {
	Value any    `json:"value"`
	Label string `json:"label"`
}

func enumOption(key, label, description string, def int) strategyOption {
	return strategyOption{
		Key:         key,
		Label:       label,
		Description: description,
		Kind:        "enum",
		ValueType:   "number",
		Values:      smaHourValues,
		Default:     def,
	}
}

func numberOption(key, label, description string, min, max, step float64, unit string, def float64) strategyOption {
	return strategyOption{
		Key:         key,
		Label:       label,
		Description: description,
		Kind:        "number",
		Min:         &min,
		Max:         &max,
		Step:        &step,
		Unit:        unit,
		Default:     def,
	}
}

func (s *Server) handleListStrategies(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"strategies": experimentReadyStrategies})
}
