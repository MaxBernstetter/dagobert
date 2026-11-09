// Package instanceparquet stores the row-heavy data of one experiment instance as Parquet files
// next to its report.json: fills, orders, fill journals, trade outcomes and period stats. One file per
// instance per datatype, written atomically, read directly by the Experiment API. The database holds
// only experiment metadata and aggregates, never rows of instance data.
//
// The package is neutral: it defines its own row types and does not import the execution store.
package instanceparquet

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/parquet-go/parquet-go"
)

// SchemaVersion is the footer schema_version of the files written by this package.
const SchemaVersion = 1

// File names inside an instance directory.
const (
	FillsFile         = "fills.parquet"
	OrdersFile        = "orders.parquet"
	JournalsFile      = "journals.parquet"
	TradeOutcomesFile = "trade_outcomes.parquet"
	PeriodStatsFile   = "period_stats.parquet"
)

// Footer key–value metadata keys, in the style of parquetmeta.
const (
	KeySchemaVersion = "trading_copilot.schema_version"
	KeyExperimentID  = "trading_copilot.experiment_id"
	KeyInstanceID    = "trading_copilot.instance_id"
	KeyPlaybookKey   = "trading_copilot.playbook_key"
	KeySymbol        = "trading_copilot.symbol"
)

// Meta identifies the instance a file belongs to. It is written to the footer of every file.
type Meta struct {
	ExperimentID string
	InstanceID   string
	PlaybookKey  string
	Symbol       string
}

// Fill is one simulated fill, as the equity curve and the trade markers read it.
type Fill struct {
	TsMs        int64   `parquet:"ts_ms,delta,snappy"`
	Side        string  `parquet:"side,dict,snappy"`
	BaseQty     float64 `parquet:"base_qty,snappy"`
	QuoteQty    float64 `parquet:"quote_qty,snappy"`
	Price       float64 `parquet:"price,snappy"`
	Fee         float64 `parquet:"fee,snappy"`
	FeeCurrency string  `parquet:"fee_currency,dict,snappy"`
}

// Order is one order result, the rows of the activity view. Optional fields are nil where the
// order body did not carry them (the JSON side used omitempty).
type Order struct {
	Ordinal      int32   `parquet:"ordinal,delta,snappy"`
	OrderLinkID  string  `parquet:"order_link_id,snappy"`
	Kind         string  `parquet:"kind,dict,snappy"`
	Playbook     string  `parquet:"playbook,dict,snappy"`
	Symbol       string  `parquet:"symbol,dict,snappy"`
	VenueSymbol  string  `parquet:"venue_symbol,dict,snappy"`
	CandleClose  int64   `parquet:"candle_close,delta,snappy"`
	Decision     *string `parquet:"decision,optional,dict,snappy"`
	OrderID      *string `parquet:"order_id,optional,snappy"`
	OrderStatus  *string `parquet:"order_status,optional,dict,snappy"`
	Side         *string `parquet:"side,optional,dict,snappy"`
	Qty          *string `parquet:"qty,optional,snappy"`
	CumExecQty   *string `parquet:"cum_exec_qty,optional,snappy"`
	CumExecValue *string `parquet:"cum_exec_value,optional,snappy"`
	AvgPrice     *string `parquet:"avg_price,optional,snappy"`
	CumExecFee   *string `parquet:"cum_exec_fee,optional,snappy"`
	FeeCurrency  *string `parquet:"fee_currency,optional,dict,snappy"`
	MarketUnit   *string `parquet:"market_unit,optional,dict,snappy"`
	RetCode      *int32  `parquet:"ret_code,optional,snappy"`
	RetMsg       *string `parquet:"ret_msg,optional,snappy"`
}

// Journal is one fill journal line with the exact strings the venue and simulator produced.
type Journal struct {
	Ordinal         int32  `parquet:"ordinal,delta,snappy"`
	OrderLinkID     string `parquet:"order_link_id,snappy"`
	Venue           string `parquet:"venue,dict,snappy"`
	VenueSymbol     string `parquet:"venue_symbol,dict,snappy"`
	Side            string `parquet:"side,dict,snappy"`
	BaseQty         string `parquet:"base_qty,snappy"`
	QuoteQty        string `parquet:"quote_qty,snappy"`
	Price           string `parquet:"price,snappy"`
	Fee             string `parquet:"fee,snappy"`
	FeeCurrency     string `parquet:"fee_currency,dict,snappy"`
	ExchangeOrderID string `parquet:"exchange_order_id,snappy"`
	PriceSource     string `parquet:"price_source,dict,snappy"`
}

// TradeOutcome is one closed trade: side 1 gain, 0 loss (int32 on disk: parquet-go has no int8);
// Percent is nil when undefined.
type TradeOutcome struct {
	Ordinal int32    `parquet:"ordinal,delta,snappy"`
	Side    int32    `parquet:"side,snappy"`
	Quote   float32  `parquet:"quote,snappy"`
	Percent *float32 `parquet:"percent,optional,snappy"`
}

// PeriodStat is one month or year bucket of an instance: its return, trade count and win count.
// Kind is "month" or "year"; Key is the period label. A nil field was not computed for the bucket.
type PeriodStat struct {
	Kind       string   `parquet:"period_kind,dict,snappy"`
	Key        string   `parquet:"period_key,snappy"`
	ReturnPct  *float64 `parquet:"return_pct,optional,snappy"`
	TradeCount *int32   `parquet:"trade_count,optional,snappy"`
	WinCount   *int32   `parquet:"win_count,optional,snappy"`
}

// ErrNotExist-style results: a missing file yields empty rows and an error for which
// errors.Is(err, fs.ErrNotExist) holds.
var ErrMissing = fs.ErrNotExist

// Remove deletes the instance files from dir. Missing files are not an error.
func Remove(dir string) error {
	var firstErr error
	for _, name := range []string{FillsFile, OrdersFile, JournalsFile, TradeOutcomesFile, PeriodStatsFile} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// footer returns the parquet writer options that embed the meta and the schema version.
func footer(meta Meta) []parquet.WriterOption {
	return []parquet.WriterOption{
		parquet.KeyValueMetadata(KeySchemaVersion, fmt.Sprint(SchemaVersion)),
		parquet.KeyValueMetadata(KeyExperimentID, meta.ExperimentID),
		parquet.KeyValueMetadata(KeyInstanceID, meta.InstanceID),
		parquet.KeyValueMetadata(KeyPlaybookKey, meta.PlaybookKey),
		parquet.KeyValueMetadata(KeySymbol, meta.Symbol),
	}
}
