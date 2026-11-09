// Package floci starts a throwaway Floci container for system tests that exercise the S3 lake.
package floci

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Start runs Floci on a free loopback port, waits until it is healthy, and returns its endpoint and an
// S3 client with a fresh bucket. The container is removed at cleanup.
func Start(t *testing.T, bucket string) (endpoint string, client *s3.Client) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	name := fmt.Sprintf("floci-system-test-%d", port)
	out, err := exec.Command("docker", "run", "-d", "--rm", "--name", name,
		"-p", fmt.Sprintf("127.0.0.1:%d:4566", port), "floci/floci:latest").CombinedOutput()
	if err != nil {
		t.Fatalf("start floci: %v\n%s", err, out)
	}
	t.Cleanup(func() { exec.Command("docker", "rm", "-f", name).Run() })
	endpoint = fmt.Sprintf("http://127.0.0.1:%d", port)
	healthy := false
	for i := 0; i < 60 && !healthy; i++ {
		if resp, err := http.Get(endpoint + "/_floci/health"); err == nil {
			resp.Body.Close()
			healthy = true
		} else {
			time.Sleep(500 * time.Millisecond)
		}
	}
	if !healthy {
		t.Fatal("floci did not become healthy")
	}
	client = s3.New(s3.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String(endpoint),
		UsePathStyle: true,
		Credentials:  credentials.NewStaticCredentialsProvider("test", "test", ""),
	})
	if _, err := client.CreateBucket(context.Background(), &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatal(err)
	}
	return endpoint, client
}
