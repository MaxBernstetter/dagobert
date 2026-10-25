package macross

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// Config is the ma-cross strategy parameter set (SMA windows in calendar hours).
type Config struct {
	FastHr int `json:"fastHr"`
	SlowHr int `json:"slowHr"`
}

// DefaultConfig is SMA(3h)/SMA(6h) — matches historical hardcoded behaviour.
func DefaultConfig() Config {
	return Config{FastHr: 3, SlowHr: 6}
}

// deployedSMAHours are windows present in TI Deployments() for algo v1.
var deployedSMAHours = map[int]struct{}{
	3: {}, 6: {}, 12: {}, 24: {}, 48: {},
}

// ParseConfig decodes JSON into Config, applying defaults for omitted fields.
// Empty or nil raw uses DefaultConfig. Unknown keys are rejected.
func ParseConfig(raw json.RawMessage) (Config, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return DefaultConfig(), nil
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return Config{}, fmt.Errorf("ma-cross config: %w", err)
	}
	for k := range probe {
		if k != "fastHr" && k != "slowHr" {
			return Config{}, fmt.Errorf("ma-cross config: unknown key %q", k)
		}
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("ma-cross config: %w", err)
	}
	return cfg, nil
}

// Validate checks window ordering, deployment availability, and aggregation divisibility.
func (c Config) Validate(aggregationDurationMS int64) error {
	if c.FastHr <= 0 || c.SlowHr <= 0 {
		return fmt.Errorf("ma-cross config: fastHr and slowHr must be positive")
	}
	if c.FastHr >= c.SlowHr {
		return fmt.Errorf("ma-cross config: fastHr (%d) must be < slowHr (%d)", c.FastHr, c.SlowHr)
	}
	if _, ok := deployedSMAHours[c.FastHr]; !ok {
		return fmt.Errorf("ma-cross config: fastHr %d not in deployed SMA set", c.FastHr)
	}
	if _, ok := deployedSMAHours[c.SlowHr]; !ok {
		return fmt.Errorf("ma-cross config: slowHr %d not in deployed SMA set", c.SlowHr)
	}
	if aggregationDurationMS > 0 {
		if aggregationsForTimeFrame(c.FastHr, aggregationDurationMS) <= 0 {
			return fmt.Errorf("ma-cross config: fastHr %d not divisible by aggregation", c.FastHr)
		}
		if aggregationsForTimeFrame(c.SlowHr, aggregationDurationMS) <= 0 {
			return fmt.Errorf("ma-cross config: slowHr %d not divisible by aggregation", c.SlowHr)
		}
	}
	return nil
}

const msPerHour = 3_600_000

func aggregationsForTimeFrame(timeFrameHr int, aggregationDurationMS int64) int {
	if aggregationDurationMS <= 0 || aggregationDurationMS%msPerHour != 0 {
		return 0
	}
	aggHr := int(aggregationDurationMS / msPerHour)
	if timeFrameHr <= 0 || aggHr <= 0 || timeFrameHr%aggHr != 0 {
		return 0
	}
	return timeFrameHr / aggHr
}

// CanonicalJSON returns stable JSON for hashing (sorted keys via struct order).
func (c Config) CanonicalJSON() ([]byte, error) {
	return json.Marshal(Config{FastHr: c.FastHr, SlowHr: c.SlowHr})
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

// AllowedSMAHours lists deployed SMA windows ascending (for UI/docs).
func AllowedSMAHours() []int {
	out := make([]int, 0, len(deployedSMAHours))
	for hr := range deployedSMAHours {
		out = append(out, hr)
	}
	sort.Ints(out)
	return out
}
