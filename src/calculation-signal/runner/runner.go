package runner

import (
	"encoding/json"

	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/playbooks"
)

// RunOneBar computes and stores the layer-1 signal for one candle close.
func RunOneBar(dataDir, playbookEnv, symbol string, candleCloseMs int64, strategyConfig json.RawMessage) (detail int, err error) {
	return playbooks.Run(dataDir, playbookEnv, symbol, candleCloseMs, strategyConfig)
}
