package runner

import (
	"github.com/MaxBernstetter/trading-copilot/src/calculation-market-action/internal/allocate"
)

// PlannedAction is a sized market-action without persisting to SQLite.
type PlannedAction struct {
	Body     []byte
	Decision string
	Size     *float64
}

// PlanAction sizes a signal and returns the action JSON body.
func PlanAction(playbookID, symbol, signalDecision, signalRule string, quoteCash, baseHeld, mark float64) (PlannedAction, error) {
	pol := allocate.PolicyFromEnv()
	out := allocate.Size(allocate.Input{
		SignalDecision: signalDecision,
		QuoteCash:      quoteCash,
		BaseHeld:       baseHeld,
		MarkPrice:      mark,
	}, pol)
	rule := signalRule
	if out.Rule != "" && out.Rule != "none" {
		rule = out.Rule
	}
	if out.Note != "" && out.Decision == "no-trade" {
		rule = out.Note
	}
	body, err := allocate.EncodeActionJSON(playbookID, symbol, out.Decision, out.Size, rule, signalRule)
	if err != nil {
		return PlannedAction{}, err
	}
	return PlannedAction{Body: body, Decision: out.Decision, Size: out.Size}, nil
}
