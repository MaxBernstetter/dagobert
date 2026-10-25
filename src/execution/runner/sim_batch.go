package runner

import (
	"github.com/MaxBernstetter/dagobert/src/execution/internal/store"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/paper"
)

// SimBatch collects simulated orders for one batched flush.
type SimBatch struct {
	orders   []store.OrderResultRow
	journals []store.Journal
}

// NewSimBatch starts an empty batch.
func NewSimBatch() *SimBatch {
	return &SimBatch{}
}

// Add appends one bar's simulation output.
func (b *SimBatch) Add(step SimBarResult) {
	b.orders = append(b.orders, step.Orders...)
	b.journals = append(b.journals, step.Journals...)
}

// Flush writes execution tables and paper wallet.
func (b *SimBatch) Flush(dataDir string, wallet *paper.Wallet) error {
	return ReplaceSimulatedExecution(dataDir, b.orders, b.journals, wallet)
}
