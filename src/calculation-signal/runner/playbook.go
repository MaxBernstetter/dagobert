package runner

import "github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/playbooks"

// ResolvePlaybook checks that playbookEnv is deployed (for backtest / CLI preflight).
func ResolvePlaybook(playbookEnv string) error {
	_, err := playbooks.Resolve(playbookEnv)
	return err
}

// PlaybookRef is a resolved registry entry (public surface for catalog/jobs).
type PlaybookRef struct {
	ID   string
	Slug string
}

// ResolvePlaybookRef maps PLAYBOOK env to registry id + slug.
func ResolvePlaybookRef(playbookEnv string) (PlaybookRef, error) {
	entry, err := playbooks.Resolve(playbookEnv)
	if err != nil {
		return PlaybookRef{}, err
	}
	return PlaybookRef{ID: entry.ID, Slug: entry.Strategy.Slug()}, nil
}

// InstancePlaybookKey builds a signal/fill playbook key that includes config identity.
func InstancePlaybookKey(entryID, configHash string) string {
	return playbooks.InstancePlaybookKey(entryID, configHash)
}
