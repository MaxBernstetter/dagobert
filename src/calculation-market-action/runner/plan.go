package runner

import (
	"github.com/MaxBernstetter/dagobert/src/calculation-market-action/internal/allocate"
)

// Policy is the allocation policy (re-exported for callers outside market-action).
type Policy = allocate.Policy

// PolicyFromEnv loads ALLOC_* sizing policy.
func PolicyFromEnv() Policy {
	return allocate.PolicyFromEnv()
}

// PlannedAction is a sized market-action without persisting to SQLite.
type PlannedAction struct {
	Body     []byte
	Decision string
	Size     *float64
}

// PlanAction sizes a signal and returns the action JSON body.
func PlanAction(playbookID, symbol, signalDecision, signalRule string, quoteCash, baseHeld, mark float64) (PlannedAction, error) {
	return PlanActionWithPolicy(playbookID, symbol, signalDecision, signalRule, quoteCash, baseHeld, mark, nil, nil, allocate.PolicyFromEnv(), true)
}

// PlanActionWithPolicy sizes a signal using a preloaded policy.
// When encodeBody is false, Body is left nil (slim experiment path).
func PlanActionWithPolicy(playbookID, symbol, signalDecision, signalRule string, quoteCash, baseHeld, mark float64, quoteNotional, baseQty *float64, pol allocate.Policy, encodeBody bool) (PlannedAction, error) {
	out := allocate.Size(allocate.Input{
		SignalDecision: signalDecision,
		QuoteCash:      quoteCash,
		BaseHeld:       baseHeld,
		MarkPrice:      mark,
		QuoteNotional:  quoteNotional,
		BaseQty:        baseQty,
	}, pol)
	rule := signalRule
	if out.Rule != "" && out.Rule != "none" {
		rule = out.Rule
	}
	if out.Note != "" && out.Decision == "no-trade" {
		rule = out.Note
	}
	planned := PlannedAction{Decision: out.Decision, Size: out.Size}
	if !encodeBody {
		return planned, nil
	}
	body, err := allocate.EncodeActionJSON(playbookID, symbol, out.Decision, out.Size, rule, signalRule)
	if err != nil {
		return PlannedAction{}, err
	}
	planned.Body = body
	return planned, nil
}
