// Package awsenv builds AWS SDK config for Floci (local) and real AWS.
package awsenv

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

// Load returns an aws.Config from the standard credential chain, so the same AWS_* environment
// variables are read on Floci and on AWS. When AWS_ENDPOINT_URL is set, callers apply Endpoint()
// as BaseEndpoint on each service client; no credentials are supplied here.
func Load(ctx context.Context) (aws.Config, error) {
	region := strings.TrimSpace(os.Getenv("AWS_REGION"))
	if region == "" {
		region = strings.TrimSpace(os.Getenv("AWS_DEFAULT_REGION"))
	}
	if region == "" {
		region = "us-east-1"
	}
	return config.LoadDefaultConfig(ctx, config.WithRegion(region))
}

// Endpoint is the Floci / LocalStack base URL, or empty for real AWS.
func Endpoint() string {
	return strings.TrimRight(strings.TrimSpace(os.Getenv("AWS_ENDPOINT_URL")), "/")
}

// ResolveEndpoint returns a BaseEndpoint pointer for SDK clients when Endpoint is set.
func ResolveEndpoint() *string {
	ep := Endpoint()
	if ep == "" {
		return nil
	}
	return aws.String(ep)
}

// LakeBucket is the S3 lake bucket (Terraform output). Empty → S3 artifacts disabled.
func LakeBucket() string {
	return strings.TrimSpace(os.Getenv("LAKE_BUCKET"))
}

// LakeBackend is "fs" (default) or "s3".
func LakeBackend() string {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("LAKE_BACKEND")))
	if v == "s3" {
		return "s3"
	}
	return "fs"
}

// LakeDualWrite is true when LAKE_DUAL_WRITE=1/true or LAKE_BACKEND=s3.
func LakeDualWrite() bool {
	if LakeBackend() == "s3" {
		return true
	}
	v := strings.ToLower(strings.TrimSpace(os.Getenv("LAKE_DUAL_WRITE")))
	return v == "1" || v == "true" || v == "yes"
}

// SSMPrefix is e.g. /dagobert/local
func SSMPrefix() string {
	p := strings.TrimSpace(os.Getenv("SSM_PREFIX"))
	if p != "" {
		return strings.TrimRight(p, "/")
	}
	env := strings.TrimSpace(os.Getenv("DEPLOY_ENV"))
	if env == "" {
		env = "local"
	}
	return "/dagobert/" + env
}

// StudyBatchSize is instances per simulate batch (default 64).
func StudyBatchSize() int {
	return intEnvDefault("EXPERIMENT_BATCH_SIZE", 64, 1)
}

// ExperimentSFNARN is the experiment fan-out state machine ARN (optional).
func ExperimentSFNARN() string {
	return strings.TrimSpace(os.Getenv("EXPERIMENT_SFN_ARN"))
}

// BackfillSFNARN is the backfill state machine (BACKFILL_SFN_ARN). Empty when the BackfillService is not served.
func BackfillSFNARN() string {
	return strings.TrimSpace(os.Getenv("BACKFILL_SFN_ARN"))
}

func intEnvDefault(key string, def, min int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < min {
		return def
	}
	return n
}

// StudyArtifactsBackend is "s3" or "fs" (default).
func StudyArtifactsBackend() string {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("EXPERIMENT_ARTIFACTS")))
	if v == "s3" {
		return "s3"
	}
	return "fs"
}
