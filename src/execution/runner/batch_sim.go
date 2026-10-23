package runner

import (
	"time"

	"github.com/MaxBernstetter/trading-copilot/src/execution/internal/store"
	"github.com/MaxBernstetter/trading-copilot/src/internal/venue/paper"
)

// ReplaceSimulatedExecution persists batched order results, fills, balances, and paper wallet.
func ReplaceSimulatedExecution(dataDir string, orders []store.OrderResultRow, journals []store.Journal, wallet *paper.Wallet) error {
	execDB, err := store.Open(dataDir)
	if err != nil {
		return err
	}
	defer execDB.Close()

	nowMs := time.Now().UnixMilli()
	balRows := make([]store.BalanceRow, 0, len(wallet.Coins))
	for _, b := range wallet.BalanceSnapshots() {
		balRows = append(balRows, store.BalanceRow{
			Currency: b.Currency, CashBalance: b.CashBalance, TransactionTime: nowMs,
		})
	}
	if err := execDB.ReplaceSimulatedRun(orders, journals, balRows); err != nil {
		return err
	}
	return wallet.Save(dataDir)
}
