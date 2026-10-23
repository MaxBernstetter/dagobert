package playbooks

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MaxBernstetter/trading-copilot/src/calculation-signal/internal/framework"
	macross "github.com/MaxBernstetter/trading-copilot/src/calculation-signal/playbooks/ma-cross"
	signalconsensus "github.com/MaxBernstetter/trading-copilot/src/calculation-signal/playbooks/signal-consensus"
)

var deployed = []framework.Entry{
	{ID: "ma-cross.yaml", Strategy: macross.Strategy{}},
	{ID: "signal-consensus.yaml", Strategy: signalconsensus.Strategy{}},
}

// Resolve maps PLAYBOOK env (slug, path, or legacy yaml path) to a registry entry.
func Resolve(playbookEnv string) (framework.Entry, error) {
	slug := normalizePlaybookRef(playbookEnv)
	for _, e := range deployed {
		if normalizePlaybookRef(e.ID) == slug || strings.TrimSuffix(e.ID, ".yaml") == slug {
			return e, nil
		}
	}
	return framework.Entry{}, fmt.Errorf("unknown playbook: %s (deployed: %s)", playbookEnv, strings.Join(deployedSlugs(), ", "))
}

func deployedSlugs() []string {
	out := make([]string, 0, len(deployed))
	for _, e := range deployed {
		out = append(out, strings.TrimSuffix(e.ID, ".yaml"))
	}
	return out
}

func normalizePlaybookRef(ref string) string {
	ref = strings.TrimSpace(ref)
	ref = strings.TrimSuffix(ref, "/")
	if ref == "" {
		return ""
	}
	ref = filepath.Base(ref)
	ref = strings.TrimSuffix(ref, ".yaml")
	return ref
}
