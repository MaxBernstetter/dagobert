# Scalability

Design every piece so it would work at **100× the data volume** (more symbols, longer history, more experiment instances), without building the 100× infrastructure today.

## Rules for every piece

- **Set-based, not row-by-row:** process data in bulk (Parquet batches, SQL statements, slices). Loops over individual rows in a store are a red flag.
- **Chunkable:** long operations take a date range or instance list and can be split into independent chunks that run in parallel (`EXPERIMENT_SIM_WORKERS`, chunked ingest).
- **Bounded memory:** stream or window large results; never load a full history into memory. The experiment path builds one shared KPI window per symbol and reuses it across instances; keep that shape.
- **Stateless and idempotent:** any run can handle any chunk, and reruns are safe (see [01-twelve-factor](01-twelve-factor.md)).
- **Query by partition key:** queries on candles and KPIs filter by symbol, year and time, so partition pruning and indexes work.
- **Low-cardinality logs and labels:** never put ids or timestamps into metric labels.
- **Bounded lists:** list endpoints in the frontend are paginated or capped.

## Balance with simplicity

Scalability is a **design property**, not an extra feature (see [03-simplicity](03-simplicity.md)):

- **Do:** choose the shape that scales (bulk, chunked, stateless) when it costs about the same as the shape that doesn't.
- **Don't:** add caches, queues, sharding or extra infrastructure until a measured need exists.

## Self-check before merging

- What happens to runtime and memory if the input is 100× larger?
- Can this run in parallel chunks without coordination?
- Does any query scan the full history unnecessarily?
