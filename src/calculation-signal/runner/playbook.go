package runner

import "github.com/MaxBernstetter/trading-copilot/src/calculation-signal/internal/playbooks"

// ResolvePlaybook checks that playbookEnv is deployed (for backtest / CLI preflight).
func ResolvePlaybook(playbookEnv string) error {
	_, err := playbooks.Resolve(playbookEnv)
	return err
}
