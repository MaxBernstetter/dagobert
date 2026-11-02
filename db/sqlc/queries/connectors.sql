-- name: GetProviderState :one
SELECT id, activated, status, error, progress_label, progress_done, progress_total, config, updated_at_ms
FROM provider
WHERE id = $1;

-- name: UpsertProviderState :exec
INSERT INTO provider (
  id, activated, status, error, progress_label, progress_done, progress_total, config, updated_at_ms
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9
)
ON CONFLICT (id) DO UPDATE SET
  activated = EXCLUDED.activated,
  status = EXCLUDED.status,
  error = EXCLUDED.error,
  progress_label = EXCLUDED.progress_label,
  progress_done = EXCLUDED.progress_done,
  progress_total = EXCLUDED.progress_total,
  config = EXCLUDED.config,
  updated_at_ms = EXCLUDED.updated_at_ms;

-- name: ListProviderPairs :many
SELECT base, quote
FROM provider_pair
WHERE provider_id = $1
ORDER BY ordinal;

-- name: DeleteProviderPairs :exec
DELETE FROM provider_pair
WHERE provider_id = $1;

-- name: InsertProviderPair :exec
INSERT INTO provider_pair (provider_id, ordinal, base, quote)
VALUES ($1, $2, $3, $4);
