package signalconsensus

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/sizing"
)

// Config is the signal-consensus threshold set (SMA windows stay fixed).
type Config struct {
	RsiBull               float64 `json:"rsiBull"`
	RsiBear               float64 `json:"rsiBear"`
	VolRatioBull          float64 `json:"volRatioBull"`
	VolRatioBear          float64 `json:"volRatioBear"`
	AtrStopMult           float64 `json:"atrStopMult"`
	MinBarsFlatAfterSell  int     `json:"minBarsFlatAfterSell"`
	MinBarsLongBeforeSell int     `json:"minBarsLongBeforeSell"`
	// Sizing is optional risk sizing; nil keeps the single target weight and full exit.
	Sizing *sizing.Config `json:"sizing,omitempty"`
}

// DefaultConfig matches historical hardcoded thresholds (logic v10 on 3h bars).
func DefaultConfig() Config {
	return Config{
		RsiBull:               defaultRsiBull,
		RsiBear:               defaultRsiBear,
		VolRatioBull:          defaultVolRatioBull,
		VolRatioBear:          defaultVolRatioBear,
		AtrStopMult:           defaultAtrStopMult,
		MinBarsFlatAfterSell:  defaultMinBarsFlatAfterSell,
		MinBarsLongBeforeSell: defaultMinBarsLongBeforeSell,
	}
}

var knownConfigKeys = map[string]struct{}{
	"rsiBull": {}, "rsiBear": {}, "volRatioBull": {}, "volRatioBear": {},
	"atrStopMult": {}, "minBarsFlatAfterSell": {}, "minBarsLongBeforeSell": {}, "sizing": {},
}

// ParseConfig decodes JSON into Config. Empty or nil raw uses DefaultConfig.
func ParseConfig(raw json.RawMessage) (Config, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return DefaultConfig(), nil
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return Config{}, fmt.Errorf("signal-consensus config: %w", err)
	}
	for k := range probe {
		if _, ok := knownConfigKeys[k]; !ok {
			return Config{}, fmt.Errorf("signal-consensus config: unknown key %q", k)
		}
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("signal-consensus config: %w", err)
	}
	if cfg.Sizing != nil {
		d := cfg.Sizing.WithDefaults()
		cfg.Sizing = &d
	}
	return cfg, nil
}

// Validate checks threshold ordering and non-negative cooldowns.
func (c Config) Validate(_ int64) error {
	if c.RsiBull <= c.RsiBear {
		return fmt.Errorf("signal-consensus config: rsiBull (%v) must be > rsiBear (%v)", c.RsiBull, c.RsiBear)
	}
	if c.RsiBull <= 0 || c.RsiBull > 100 || c.RsiBear < 0 || c.RsiBear >= 100 {
		return fmt.Errorf("signal-consensus config: RSI thresholds must be in (0,100]")
	}
	if c.VolRatioBull <= 0 || c.VolRatioBear <= 0 {
		return fmt.Errorf("signal-consensus config: volume ratios must be positive")
	}
	if c.VolRatioBull <= c.VolRatioBear {
		return fmt.Errorf("signal-consensus config: volRatioBull (%v) must be > volRatioBear (%v)", c.VolRatioBull, c.VolRatioBear)
	}
	if c.AtrStopMult <= 0 {
		return fmt.Errorf("signal-consensus config: atrStopMult must be positive")
	}
	if c.Sizing != nil {
		if err := c.Sizing.Validate(); err != nil {
			return fmt.Errorf("signal-consensus config: %w", err)
		}
	}
	if c.MinBarsFlatAfterSell < 0 || c.MinBarsLongBeforeSell < 0 {
		return fmt.Errorf("signal-consensus config: cooldown bars must be >= 0")
	}
	return nil
}

// CanonicalJSON returns stable JSON for hashing.
func (c Config) CanonicalJSON() ([]byte, error) {
	return json.Marshal(Config{
		RsiBull:               c.RsiBull,
		RsiBear:               c.RsiBear,
		VolRatioBull:          c.VolRatioBull,
		VolRatioBear:          c.VolRatioBear,
		AtrStopMult:           c.AtrStopMult,
		MinBarsFlatAfterSell:  c.MinBarsFlatAfterSell,
		MinBarsLongBeforeSell: c.MinBarsLongBeforeSell,
		Sizing:                c.Sizing,
	})
}

// Hash returns a hex SHA-256 of the canonical config JSON.
func (c Config) Hash() (string, error) {
	body, err := c.CanonicalJSON()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}
