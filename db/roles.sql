-- Least-privilege application role. Run as the master user after `atlas migrate apply`
-- (docker/db-migrate/entrypoint.sh and `task db:migrate`). Idempotent.
--
-- Usage: psql "$MASTER_URL" -v ON_ERROR_STOP=1 -v app_password="$APP_DATABASE_PASSWORD" -f db/roles.sql
--
-- The application (Lambdas, ECS jobs) connects as dagobert_app: DML on the public schema only.
-- Migrations (DDL) stay on the master user, so the app cannot CREATE or ALTER tables.

-- Create the role on first run; set the password on every run so rotation is one apply.
SELECT format('CREATE ROLE dagobert_app LOGIN PASSWORD %L', :'app_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'dagobert_app')
\gexec
SELECT format('ALTER ROLE dagobert_app LOGIN PASSWORD %L', :'app_password') \gexec

GRANT USAGE ON SCHEMA public TO dagobert_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO dagobert_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO dagobert_app;

-- Tables and sequences created later by the master user (the next migrations) get the same grants.
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO dagobert_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO dagobert_app;
