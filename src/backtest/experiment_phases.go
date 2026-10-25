package backtest

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ExperimentPhase is all|prepare|simulate|finalize (default all).
func ExperimentPhase() string {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("EXPERIMENT_PHASE")))
	switch v {
	case "", "all":
		return "all"
	case "prepare", "simulate", "finalize":
		return v
	default:
		return "all"
	}
}

// ResolveExperimentInstanceIDs reads instance ids from EXPERIMENT_INSTANCE_IDS, EXPERIMENT_INSTANCE_FILE, or argv.
func ResolveExperimentInstanceIDs() ([]string, error) {
	if path := strings.TrimSpace(os.Getenv("EXPERIMENT_INSTANCE_FILE")); path != "" {
		return readInstanceIDFile(path)
	}
	raw := strings.TrimSpace(os.Getenv("EXPERIMENT_INSTANCE_IDS"))
	if raw == "" && len(os.Args) > 1 {
		// experiment-worker / backtest may pass ids as trailing args after flags.
		for _, arg := range os.Args[1:] {
			if strings.Contains(arg, ",") && !strings.HasPrefix(arg, "-") {
				raw = arg
				break
			}
		}
	}
	if raw == "" {
		return nil, fmt.Errorf("EXPERIMENT_INSTANCE_IDS or EXPERIMENT_INSTANCE_FILE required for simulate phase")
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		id := strings.TrimSpace(part)
		if id != "" {
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no instance ids in EXPERIMENT_INSTANCE_IDS")
	}
	return out, nil
}

func readInstanceIDFile(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(raw))
	if strings.HasPrefix(text, "[") {
		var ids []string
		if err := json.Unmarshal(raw, &ids); err != nil {
			return nil, err
		}
		return ids, nil
	}
	var out []string
	for _, line := range strings.Split(text, "\n") {
		id := strings.TrimSpace(line)
		if id != "" && !strings.HasPrefix(id, "#") {
			out = append(out, id)
		}
	}
	return out, nil
}
