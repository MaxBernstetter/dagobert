package playbook

import (
	"encoding/json"

	"github.com/MaxBernstetter/trading-copilot/strategy/internal/canonical"
)

// KPIResult is one indicator value from a KPI row body.
type KPIResult struct {
	Def   json.RawMessage
	Value json.RawMessage
	Unit  string
}

// Decision is the outcome of walking playbook rules.
type Decision struct {
	Decision string
	Size     *float64
	Exit     *Exit
	RuleID   string
}

// Evaluate walks rules in order and returns the first match, or an explicit no-trade.
func Evaluate(doc *Document, results []KPIResult) (Decision, error) {
	index, err := buildResultIndex(results)
	if err != nil {
		return Decision{}, err
	}

	for _, rule := range doc.Rules {
		want, err := rule.When.Def.String()
		if err != nil {
			return Decision{}, err
		}
		rawValue, ok := index[want]
		if !ok {
			continue
		}
		fv, ok := jsonNumberToFloat64(rawValue)
		if !ok {
			continue
		}
		if !Compare(rule.When, fv) {
			continue
		}
		return decisionFromRule(rule)
	}

	return Decision{
		Decision: "no-trade",
		Size:     nil,
		Exit:     nil,
		RuleID:   "none",
	}, nil
}

func buildResultIndex(results []KPIResult) (map[string]json.RawMessage, error) {
	index := make(map[string]json.RawMessage, len(results))
	for _, r := range results {
		def, err := canonical.ParseDefFromJSON(r.Def)
		if err != nil {
			continue
		}
		key, err := def.String()
		if err != nil {
			return nil, err
		}
		index[key] = r.Value
	}
	return index, nil
}

func decisionFromRule(rule Rule) (Decision, error) {
	if rule.Decision == "no-trade" {
		return Decision{
			Decision: "no-trade",
			Size:     nil,
			Exit:     nil,
			RuleID:   rule.ID,
		}, nil
	}
	size := rule.Size
	exit := rule.Exit
	return Decision{
		Decision: rule.Decision,
		Size:     &size,
		Exit:     exit,
		RuleID:   rule.ID,
	}, nil
}

func jsonNumberToFloat64(raw json.RawMessage) (float64, bool) {
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f, true
	}
	var i int64
	if err := json.Unmarshal(raw, &i); err == nil {
		return float64(i), true
	}
	return 0, false
}
