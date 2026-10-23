package runner

import (
	"github.com/MaxBernstetter/trading-copilot/src/calculation-signal/internal/playbooks"
)

// RunOneBar computes and stores the layer-1 signal for one candle close.
func RunOneBar(dataDir, playbookEnv, symbol string, candleCloseMs int64) (detail int, err error) {
	return playbooks.Run(dataDir, playbookEnv, symbol, candleCloseMs)
}
