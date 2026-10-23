package run

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/indicators"
	"gopkg.in/yaml.v3"
)

type readmeSeriesEntry struct {
	Key        string                 `yaml:"key"`
	Definition map[string]interface{} `yaml:"definition"`
}

func indicatorsRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(file))
}

func TestWikiREADMESeriesMatchCode(t *testing.T) {
	root := indicatorsRoot()
	bySlug := map[string][]readmeSeriesEntry{}
	for _, dep := range Deployments() {
		slug := dep.Indicator.Slug()
		bySlug[slug] = append(bySlug[slug], readmeSeriesEntry{
			Key:        dep.Key,
			Definition: definitionMap(dep.Indicator, dep.TimeFrameHr),
		})
	}
	seen := map[string]bool{}
	for _, dep := range Deployments() {
		slug := dep.Indicator.Slug()
		if seen[slug] {
			continue
		}
		seen[slug] = true
		path := filepath.Join(root, slug, "README.md")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: read README: %v", slug, err)
		}
		front, err := splitFrontMatter(string(raw))
		if err != nil {
			t.Fatalf("%s: %v", slug, err)
		}
		var doc struct {
			Series []readmeSeriesEntry `yaml:"series"`
		}
		if err := yaml.Unmarshal([]byte(front), &doc); err != nil {
			t.Fatalf("%s: parse frontmatter: %v", slug, err)
		}
		want := bySlug[slug]
		if len(doc.Series) != len(want) {
			t.Fatalf("%s: README lists %d series, deployments have %d", slug, len(doc.Series), len(want))
		}
		byKey := map[string]readmeSeriesEntry{}
		for _, row := range doc.Series {
			byKey[row.Key] = row
		}
		for _, row := range want {
			got, ok := byKey[row.Key]
			if !ok {
				t.Fatalf("%s: missing README series for key %s", slug, row.Key)
			}
			if !reflect.DeepEqual(normalizeNumbers(row.Definition), normalizeNumbers(got.Definition)) {
				t.Fatalf("%s %s: README definition %#v != code %#v", slug, row.Key, got.Definition, row.Definition)
			}
		}
	}
}

func definitionMap(ind indicators.TechnicalIndicator, timeFrameHr int) map[string]interface{} {
	raw, _ := json.Marshal(ind.Definition(timeFrameHr))
	var out map[string]interface{}
	_ = json.Unmarshal(raw, &out)
	return out
}

func splitFrontMatter(raw string) (string, error) {
	parts := strings.SplitN(raw, "---", 3)
	if len(parts) < 3 {
		return "", errFrontMatter
	}
	return strings.TrimSpace(parts[1]), nil
}

var errFrontMatter = &frontMatterError{}

type frontMatterError struct{}

func (frontMatterError) Error() string { return "missing YAML frontmatter" }

func normalizeNumbers(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		switch n := v.(type) {
		case int:
			out[k] = float64(n)
		case int64:
			out[k] = float64(n)
		case float64:
			out[k] = n
		default:
			out[k] = v
		}
	}
	return out
}
