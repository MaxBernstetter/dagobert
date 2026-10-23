package signalstore

import (
	"os"
	"path/filepath"
)

const fileName = "calculation_signal.sqlite"
const legacyFileName = "strategy.sqlite"

// SQLitePath returns the signal/action store path under dataDir. Prefer
// calculation_signal.sqlite; use strategy.sqlite when the legacy file exists.
func SQLitePath(dataDir string) string {
	current := filepath.Join(dataDir, fileName)
	if _, err := os.Stat(current); err == nil {
		return current
	}
	legacy := filepath.Join(dataDir, legacyFileName)
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}
	return current
}
