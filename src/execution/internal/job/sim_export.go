package job

import "github.com/MaxBernstetter/dagobert/src/execution/internal/store"

// OrderLinkID builds the canonical order link id for a playbook bar.
func OrderLinkID(playbook, symbol string, candleClose int64) string {
	return orderLinkID(playbook, symbol, candleClose)
}

// JournalFromFill maps a fill key to fill_journal columns.
func JournalFromFill(key KeyFields) store.Journal {
	return journalFrom(key)
}

// FormatOrderQty formats quantities for order bodies.
func FormatOrderQty(f float64) string {
	return formatQty(f)
}

// VenueSymbolForSeries maps ingest symbol to a paper/sim venue symbol.
func VenueSymbolForSeries(symbol string) (string, bool) {
	return paperVenueSymbol(symbol)
}
