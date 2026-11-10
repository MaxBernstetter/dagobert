#!/bin/sh
# Applies db/migrations to DATABASE_URL, the same as `task db:migrate` (atlas migrate apply --env local).
# Exit 0: migrations applied or already up to date. Non-zero: connection or migration failure.
set -eu

if [ -z "${DATABASE_URL:-}" ]; then
  echo "db-migrate: DATABASE_URL is not set" >&2
  exit 1
fi
cd /db

# Some migrations REVOKE from the Supabase roles anon and authenticated. Those roles do not exist on Aurora
# or on a plain Postgres, so create them (NOLOGIN, no grants) before Atlas runs. Same as `task db:up` locally.
# libpq rejects the Atlas-only search_path parameter, so strip it for psql.
PSQL_URL="$(printf '%s' "$DATABASE_URL" | sed -e 's/[?&]search_path=[^&]*//' -e 's/?&/?/')"
psql "$PSQL_URL" -v ON_ERROR_STOP=1 -q -c "DO \$\$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN CREATE ROLE anon NOLOGIN; END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN CREATE ROLE authenticated NOLOGIN; END IF;
END \$\$"

echo "db-migrate: applying migrations"
atlas migrate apply --env local
echo "db-migrate: status after apply"
atlas migrate status --env local
