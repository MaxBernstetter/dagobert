package awsenv

import "testing"

func TestExperimentCatalogPostgresWhenDatabaseURLSet(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://trading:trading@127.0.0.1:5432/trading?sslmode=disable")
	t.Setenv("EXPERIMENT_CATALOG", "ddb")
	t.Setenv("EXPERIMENTS_TABLE", "dagobert-local-experiments")

	if got := ExperimentCatalogBackend(); got != "postgres" {
		t.Fatalf("backend: got %q want postgres", got)
	}
}

func TestExperimentCatalogIgnoresDynamoDBWhenDatabaseURLUnset(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("EXPERIMENT_CATALOG", "ddb")
	t.Setenv("EXPERIMENTS_TABLE", "dagobert-local-experiments")

	if got := ExperimentCatalogBackend(); got != "sqlite" {
		t.Fatalf("backend: got %q want sqlite", got)
	}
}

func TestExperimentCatalogSQLiteWhenDatabaseURLUnset(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("EXPERIMENT_CATALOG", "postgres")

	if got := ExperimentCatalogBackend(); got != "sqlite" {
		t.Fatalf("backend: got %q want sqlite", got)
	}
}
