package playbook

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"

	"github.com/MaxBernstetter/trading-copilot/services/strategy/internal/canonical"
)

// ErrSchemaValidation means the playbook YAML failed JSON Schema lint.
var ErrSchemaValidation = errors.New("playbook schema validation failed")

// ErrSchemaLoad means the JSON Schema could not be loaded or compiled.
var ErrSchemaLoad = errors.New("playbook schema load failed")

const threeHoursMs = 3 * 60 * 60 * 1000

// IndicatorDef re-exports the canonical indicator definition.
type IndicatorDef = canonical.IndicatorDef

// When is a single indicator comparison against a literal.
type When struct {
	Def   IndicatorDef `json:"def" yaml:"def"`
	Op    string       `json:"op" yaml:"op"`
	Value float64      `json:"value" yaml:"value"`
}

// Exit describes how to leave a trade.
type Exit struct {
	Kind string `json:"kind" yaml:"kind"`
	Bars int    `json:"bars" yaml:"bars"`
}

// Rule is one playbook rule in evaluation order.
type Rule struct {
	ID       string  `json:"id" yaml:"id"`
	When     When    `json:"when" yaml:"when"`
	Decision string  `json:"decision" yaml:"decision"`
	Size     float64 `json:"size,omitempty" yaml:"size,omitempty"`
	Exit     *Exit   `json:"exit,omitempty" yaml:"exit,omitempty"`
}

// Document is a validated playbook.
type Document struct {
	Name  string `json:"name" yaml:"name"`
	Rules []Rule `json:"rules" yaml:"rules"`
}

// LoadAndValidate reads PLAYBOOK path YAML and validates it against the JSON Schema.
func LoadAndValidate(path string, compiler *jsonschema.Compiler) (*Document, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read playbook: %w", err)
	}

	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	jsonBytes, err := yamlNodeToJSON(&node)
	if err != nil {
		return nil, fmt.Errorf("yaml to json: %w", err)
	}

	schema, err := compileSchema(compiler)
	if err != nil {
		return nil, err
	}
	var docJSON any
	if err := json.Unmarshal(jsonBytes, &docJSON); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	if err := validateDocument(schema, docJSON); err != nil {
		return nil, err
	}

	var doc Document
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decode playbook: %w", err)
	}
	return &doc, nil
}

func validateDocument(schema *jsonschema.Schema, docJSON any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: %v", ErrSchemaLoad, r)
		}
	}()
	if err := schema.Validate(docJSON); err != nil {
		return fmt.Errorf("%w: %v", ErrSchemaValidation, err)
	}
	return nil
}

// BarOpen returns the 3-hour bar open time for a candle close timestamp.
func BarOpen(candleCloseMs int64) int64 {
	return candleCloseMs - threeHoursMs
}

// Compare applies the when clause to an indicator value.
func Compare(w When, indicatorValue float64) bool {
	switch w.Op {
	case "gt":
		return indicatorValue > w.Value
	case "gte":
		return indicatorValue >= w.Value
	case "lt":
		return indicatorValue < w.Value
	case "lte":
		return indicatorValue <= w.Value
	case "eq":
		return indicatorValue == w.Value
	default:
		return false
	}
}

func yamlNodeToJSON(node *yaml.Node) ([]byte, error) {
	var v any
	if err := node.Decode(&v); err != nil {
		return nil, err
	}
	normalized := normalizeYAML(v)
	return json.Marshal(normalized)
}

func normalizeYAML(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = normalizeYAML(val)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[fmt.Sprint(k)] = normalizeYAML(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = normalizeYAML(val)
		}
		return out
	default:
		return t
	}
}
