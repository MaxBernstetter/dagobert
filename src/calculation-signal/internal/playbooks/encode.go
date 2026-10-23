package playbooks

import (
	"github.com/MaxBernstetter/trading-copilot/src/calculation-signal/internal/framework"
)

// EncodeSignalJSON builds the signal row body for storage.
func EncodeSignalJSON(entry framework.Entry, symbol string, dec framework.Decision) ([]byte, error) {
	return encodeSignal(entry, symbol, dec)
}
