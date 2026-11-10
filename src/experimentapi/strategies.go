package experimentapi

import (
	"context"
	"strconv"

	"connectrpc.com/connect"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

// experimentReadyStrategies lists the playbooks that accept a strategyConfig and can be replayed
// by the experiment runner, with the options the create form renders for each. It is an allowlist:
// add a playbook here only when the runner can validate and replay its config.
var experimentReadyStrategies = []*v1.ExperimentStrategy{
	{
		Id:              "ma-cross",
		Label:           "Moving-Average Cross",
		ExperimentReady: true,
		Options: []*v1.StrategyOption{
			enumOption("fastHr", "Fast window", "Shorter SMA lookback in calendar hours (must divide bar duration).", 3),
			enumOption("slowHr", "Slow window", "Longer SMA lookback; must be greater than the fast window.", 6),
		},
	},
	{
		Id:              "signal-consensus",
		Label:           "Signal consensus",
		ExperimentReady: true,
		Options: []*v1.StrategyOption{
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

var smaHourValues = func() []*v1.StrategyOptionValue {
	hours := []float64{3, 6, 12, 24, 48}
	out := make([]*v1.StrategyOptionValue, 0, len(hours))
	for _, h := range hours {
		out = append(out, &v1.StrategyOptionValue{Value: structpb.NewNumberValue(h), Label: strconv.FormatFloat(h, 'f', -1, 64) + "h"})
	}
	return out
}()

func enumOption(key, label, description string, def float64) *v1.StrategyOption {
	return &v1.StrategyOption{
		Key:         key,
		Label:       label,
		Description: description,
		Kind:        "enum",
		ValueType:   "number",
		Values:      smaHourValues,
		Default:     structpb.NewNumberValue(def),
	}
}

func numberOption(key, label, description string, min, max, step float64, unit string, def float64) *v1.StrategyOption {
	return &v1.StrategyOption{
		Key:         key,
		Label:       label,
		Description: description,
		Kind:        "number",
		Min:         &min,
		Max:         &max,
		Step:        &step,
		Unit:        unit,
		Default:     structpb.NewNumberValue(def),
	}
}

func (s *Server) ListStrategies(context.Context, *connect.Request[v1.ListStrategiesRequest]) (*connect.Response[v1.ListStrategiesResponse], error) {
	return connect.NewResponse(&v1.ListStrategiesResponse{Strategies: experimentReadyStrategies}), nil
}
