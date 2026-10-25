package framework

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/sizing"
)

type journalFill struct {
	closeMs int64
	side    string
	baseQty float64
	price   float64
}

// DeriveOpenUnits rebuilds the open sized units before beforeCandleClose from the fill
// journal and the stored signals, with the same rules as the simulated pipeline: a buy
// signal with a stop price adds a unit at the fill, a pyramid raises earlier stops (they
// never fall), a partial sell scales every unit, and a full exit or a remainder worth
// less than minOrderQuote clears them. Buys without a stop (legacy sizing) add nothing.
func DeriveOpenUnits(dataDir string, sigDB *sql.DB, playbookID, symbol string, beforeCandleClose int64, minOrderQuote float64) ([]sizing.Unit, error) {
	fills, err := loadJournalFills(dataDir, playbookID, symbol, beforeCandleClose)
	if err != nil || len(fills) == 0 {
		return nil, err
	}
	var units []sizing.Unit
	var base float64
	for _, f := range fills {
		sig, err := ReadSignal(sigDB, playbookID, symbol, f.closeMs)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		before := base
		switch f.side {
		case "buy":
			base += f.baseQty
			if sig.RaiseStopsBy != nil && *sig.RaiseStopsBy > 0 {
				for i := range units {
					units[i].Stop += *sig.RaiseStopsBy
				}
			}
			if sig.StopPrice != nil {
				units = append(units, sizing.Unit{Fill: f.price, Stop: *sig.StopPrice, Qty: f.baseQty})
			}
		case "sell":
			base = max(0, base-f.baseQty)
			if sig.BaseQty == nil || before <= 0 || base <= 0 || base*f.price < minOrderQuote {
				units = nil
				continue
			}
			keep := base / before
			for i := range units {
				units[i].Qty *= keep
			}
		}
	}
	return units, nil
}

func loadJournalFills(dataDir, playbookID, symbol string, beforeCandleClose int64) ([]journalFill, error) {
	path := filepath.Join(dataDir, "execution.sqlite")
	if _, err := os.Stat(path); err != nil {
		return nil, nil
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT side, order_link_id, base_qty, price FROM fill_journal WHERE order_link_id LIKE ?`, playbookID+":"+symbol+":%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []journalFill
	for rows.Next() {
		var side, link, baseS, priceS string
		if err := rows.Scan(&side, &link, &baseS, &priceS); err != nil {
			return nil, err
		}
		closeMs, ok := candleCloseFromLink(link, playbookID, symbol)
		if !ok || closeMs >= beforeCandleClose {
			continue
		}
		q, _ := strconv.ParseFloat(strings.TrimSpace(baseS), 64)
		p, _ := strconv.ParseFloat(strings.TrimSpace(priceS), 64)
		out = append(out, journalFill{closeMs: closeMs, side: strings.ToLower(side), baseQty: q, price: p})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].closeMs < out[j].closeMs })
	return out, nil
}
