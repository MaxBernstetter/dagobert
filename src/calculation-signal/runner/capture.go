package runner

import (
	"strconv"
	"strings"

	execrunner "github.com/MaxBernstetter/dagobert/src/execution/runner"
)

// FillsFromActivity derives the report fills from captured fill journals. Each journal takes the
// candle close of the order with the same order link id, and the quantities are read back from
// the journal strings exactly as ReplayKPI does.
func FillsFromActivity(act *execrunner.Activity) []StrategyKPIFill {
	if act == nil {
		return nil
	}
	closeByLink := make(map[string]int64, len(act.Orders))
	for _, o := range act.Orders {
		closeByLink[o.OrderLinkID] = o.CandleClose
	}
	fills := make([]StrategyKPIFill, 0, len(act.Journals))
	for _, j := range act.Journals {
		base, _ := strconv.ParseFloat(strings.TrimSpace(j.BaseQty), 64)
		quote, _ := strconv.ParseFloat(strings.TrimSpace(j.QuoteQty), 64)
		price, _ := strconv.ParseFloat(strings.TrimSpace(j.Price), 64)
		fills = append(fills, StrategyKPIFill{
			CandleCloseMs: closeByLink[j.OrderLinkID], Side: strings.ToLower(j.Side),
			BaseQty: base, QuoteQty: quote, Price: price,
		})
	}
	return fills
}
