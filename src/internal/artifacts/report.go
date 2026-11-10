// Package artifacts stores experiment instance report JSON on S3 (Floci or AWS).
package artifacts

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/MaxBernstetter/dagobert/src/internal/awsenv"
)

const ReportFileName = "report.json"

// ObjectKey builds the S3 key from a workspace-relative path
// (e.g. experiments/{experimentId}/instances/{instanceId}/report.json).
func ObjectKey(workspaceRelPath string) string {
	rel := strings.Trim(strings.ReplaceAll(workspaceRelPath, "\\", "/"), "/")
	if rel == "" {
		return ReportFileName
	}
	return rel + "/" + ReportFileName
}

// Enabled is true when EXPERIMENT_ARTIFACTS=s3 and LAKE_BUCKET is set.
func Enabled() bool {
	return awsenv.StudyArtifactsBackend() == "s3" && awsenv.LakeBucket() != ""
}

// PutBytes writes raw report JSON to S3.
func PutBytes(ctx context.Context, key string, body []byte) error {
	bucket := awsenv.LakeBucket()
	if bucket == "" {
		return fmt.Errorf("LAKE_BUCKET required")
	}
	cfg, err := awsenv.Load(ctx)
	if err != nil {
		return err
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if ep := awsenv.ResolveEndpoint(); ep != nil {
			o.BaseEndpoint = ep
			o.UsePathStyle = true
		}
	})
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String("application/json"),
	})
	return err
}
