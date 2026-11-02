# Simplicity

Build the simplest thing that fully solves the problem. Complexity is added only when a real requirement forces it.

## Rules

- **YAGNI:** implement what the current issue needs. No speculative options, hooks or abstractions.
- **KISS:** prefer plain, boring solutions: a SQL query over a framework, a function over a type hierarchy, a config value over a plugin system.
- **One reason to exist:** every package, job and table has one clear responsibility you can state in a sentence.
- **Deep, not wide:** small interfaces with real behaviour behind them. If a layer only passes calls through, remove it.
- **Two cases before abstracting:** don't abstract over one implementation. Wait for the second real one (rule of three for shared helpers). The indicator and playbook registries are the existing pattern for plugins.
- **Delete freely:** dead code, unused flags and obsolete paths are removed, not commented out.

## Simplicity vs. the architecture

The system deliberately accepts some structure (stateless job containers, an orchestrator, a shared data volume) because the requirements demand it. Within that structure, each piece should be as simple as possible:

- Use the standard interfaces that exist (the exit-code contract, the Parquet layout in `src/shared/`) rather than inventing variants.
- Use the platform (the orchestrator's fan-out and exit-code contract) rather than re-implementing scheduling or retries inside a job.

## Self-check

- Could a new team member understand this piece in ten minutes?
- What can I remove and still meet the acceptance criteria?
- Is any part here "just in case"?
