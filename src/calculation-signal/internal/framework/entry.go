package framework

// Entry binds a deployed playbook id to its implementation.
type Entry struct {
	ID       string
	Strategy Strategy
}
