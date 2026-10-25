package main

import (
	"os"
	"strings"
)

// dockerDataPlaneEnvArgs passes lake/catalog AWS env into job containers.
// When DATABASE_URL is set, experiment batches use the Postgres catalog and do not
// receive EXPERIMENTS_TABLE (DynamoDB is not part of the experiment path).
func dockerDataPlaneEnvArgs() []string {
	postgres := strings.TrimSpace(os.Getenv("DATABASE_URL")) != ""
	var args []string
	for _, key := range []string{
		"AWS_ENDPOINT_URL", "AWS_REGION", "AWS_DEFAULT_REGION",
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
		"LAKE_BUCKET", "EXPERIMENT_ARTIFACTS", "DEPLOY_ENV", "SSM_PREFIX",
		"EXPERIMENTS_TABLE", "EXPERIMENT_CATALOG", "LAKE_BACKEND", "LAKE_DUAL_WRITE",
		"EXPERIMENT_RUN_BACKEND", "EXPERIMENT_BATCH_SIZE", "EXPERIMENT_MAP_MAX_CONCURRENCY", "EXPERIMENT_SFN_ARN",
		"DATABASE_URL",
	} {
		if postgres && (key == "EXPERIMENTS_TABLE" || key == "EXPERIMENT_CATALOG") {
			continue
		}
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			args = append(args, "-e", key+"="+v)
		}
	}
	if postgres {
		args = append(args, "-e", "EXPERIMENT_CATALOG=postgres")
	}
	return args
}
