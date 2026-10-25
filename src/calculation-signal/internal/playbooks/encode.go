package playbooks

import (
	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
)

// EncodeSignalJSON builds the signal row body for storage.
func EncodeSignalJSON(entry framework.Entry, symbol string, dec framework.Decision) ([]byte, error) {
	return encodeSignal(entry, symbol, dec)
}

// EncodeSignalJSONWithConfig includes canonical strategy config in playbook_hash.
func EncodeSignalJSONWithConfig(entry framework.Entry, symbol string, dec framework.Decision, configJSON []byte) ([]byte, error) {
	return encodeSignalWithConfig(entry, symbol, dec, configJSON)
}

// InstancePlaybookKey builds a signal/fill playbook key that includes config identity.
func InstancePlaybookKey(entryID, configHash string) string {
	if configHash == "" {
		return entryID
	}
	return entryID + "#" + configHash
}
