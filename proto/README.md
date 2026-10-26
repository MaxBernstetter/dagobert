# proto

Protobuf schemas shared by the Go services, the C++ engine and the frontend. Package `tradingcopilot.v1` in `proto/tradingcopilot/v1/`, module `buf.build/maxbernstetter/dagobert`.

## Files
Experiments and API shapes: `experiment*.proto`, `provider.proto`, `strategy.proto`. C++ engine contract: `sweep.proto` (`SweepRequest` in), `run_report.proto` and `run_envelope.proto` (report out).

## Tasks
```bash
task proto:lint       # buf lint (STANDARD)
task proto:gen        # generate Go, TS and C++
task proto:breaking   # buf breaking (FILE) against main, or the BSR if pushed
task proto:push       # push to the Buf Schema Registry (buf registry login first)
```
Run lint and breaking before changing a schema.

## Generated code (committed)
- Go: `src/gen/` (`go_package_prefix` is set in `buf.gen.yaml`).
- TypeScript: `src/frontend/lib/gen/` (`protobuf-es`).
- C++: `src/calculation/gen/cpp/`.

## Breaking-change exception
`buf.yaml` ignores rule `FIELD_SAME_CARDINALITY` for `tradingcopilot/v1/strategy.proto` only. Issue #128 moved fields to explicit presence; nothing is deployed against the old schema yet. Remove the exception once the schema is stable.

## Before changing
Never edit generated files; regenerate and commit them with the `.proto` change. The C++ engine and the Go replay code both read `sweep.proto` and the report messages, so change them together. Background: `docs/development/protobuf.md`.
