// Package workspace stores experiment workspace data (result shards, the prepare cache) under
// keys relative to DATA_DIR, for example experiments/<id>/shards/<shard>/reports.parquet.
//
// With LAKE_BACKEND=fs (default) a key is the file dataDir/<key>. With LAKE_BACKEND=s3 the object
// with that key in LAKE_BUCKET is the only store: writes must succeed in the bucket, reads download
// to a temporary file, and nothing stays under dataDir. The same switch and bucket serve the
// candle lake (src/internal/lakeobject).
package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/MaxBernstetter/dagobert/src/internal/awsenv"
)

const (
	callTimeout = 2 * time.Minute
	// deleteBatch is the S3 limit of keys per DeleteObjects call.
	deleteBatch = 1000
)

// Enabled reports whether the workspace lives in the S3 bucket (LAKE_BACKEND=s3).
func Enabled() bool { return awsenv.LakeBackend() == "s3" }

// File is a readable workspace file. In s3 mode it is a temporary download that Close removes.
type File struct {
	*os.File
	remove string
}

// Close closes the file and removes a temporary download.
func (f *File) Close() error {
	err := f.File.Close()
	if f.remove != "" {
		_ = os.Remove(f.remove)
	}
	return err
}

// Open opens the key for reading and returns the file with its size. A missing key returns an
// error satisfying errors.Is(err, fs.ErrNotExist) in both modes. In s3 mode the whole object is
// downloaded; shard files are a few MB, so there are no range reads.
func Open(ctx context.Context, dataDir, key string) (*File, int64, error) {
	local, err := localPath(dataDir, key)
	if err != nil {
		return nil, 0, err
	}
	if !Enabled() {
		f, err := os.Open(local)
		if err != nil {
			return nil, 0, err
		}
		st, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, 0, err
		}
		return &File{File: f}, st.Size(), nil
	}
	cli, bucket, err := client(ctx)
	if err != nil {
		return nil, 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	out, err := cli.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return nil, 0, wrapNotFound(key, err)
	}
	defer out.Body.Close()
	tmp, err := os.CreateTemp("", "workspace-*")
	if err != nil {
		return nil, 0, err
	}
	n, err := io.Copy(tmp, out.Body)
	if err == nil {
		_, err = tmp.Seek(0, io.SeekStart)
	}
	if err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, 0, fmt.Errorf("download %s: %w", key, err)
	}
	return &File{File: tmp, remove: tmp.Name()}, n, nil
}

// ReadBytes returns the content of the key. A missing key satisfies errors.Is(err, fs.ErrNotExist).
func ReadBytes(ctx context.Context, dataDir, key string) ([]byte, error) {
	f, _, err := Open(ctx, dataDir, key)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// WriteBytes stores body under the key, replacing an existing object or file.
func WriteBytes(ctx context.Context, dataDir, key string, body []byte) error {
	local, err := localPath(dataDir, key)
	if err != nil {
		return err
	}
	if !Enabled() {
		if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
			return err
		}
		return os.WriteFile(local, body, 0o644)
	}
	return put(ctx, key, body)
}

// PublishDir makes a directory freshly written at dataDir/<keyPrefix> durable. With fs it is
// already in place. With s3 every file in it is uploaded under keyPrefix/<name>, and the local
// directory is removed afterwards. If an upload fails nothing of the directory stays in the bucket
// and the error is returned.
func PublishDir(ctx context.Context, dataDir, keyPrefix string) error {
	if !Enabled() {
		return nil
	}
	local, err := localPath(dataDir, keyPrefix)
	if err != nil {
		return err
	}
	defer os.RemoveAll(local)
	entries, err := os.ReadDir(local)
	if err != nil {
		return err
	}
	for _, e := range entries {
		body, err := os.ReadFile(filepath.Join(local, e.Name()))
		if err == nil {
			err = put(ctx, path.Join(keyPrefix, e.Name()), body)
		}
		if err != nil {
			if derr := DeletePrefix(ctx, dataDir, keyPrefix); derr != nil {
				log.Printf("workspace: remove partial %s: %v", keyPrefix, derr)
			}
			return fmt.Errorf("upload %s: %w", keyPrefix, err)
		}
	}
	return nil
}

// List returns the names of the files directly under keyPrefix, sorted. A missing prefix is empty.
func List(ctx context.Context, dataDir, keyPrefix string) ([]string, error) {
	local, err := localPath(dataDir, keyPrefix)
	if err != nil {
		return nil, err
	}
	if !Enabled() {
		entries, err := os.ReadDir(local)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		var names []string
		for _, e := range entries {
			if !e.IsDir() {
				names = append(names, e.Name())
			}
		}
		return names, nil
	}
	keys, err := listKeys(ctx, keyPrefix+"/")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, k := range keys {
		if name := strings.TrimPrefix(k, keyPrefix+"/"); !strings.Contains(name, "/") {
			names = append(names, name)
		}
	}
	return names, nil
}

// DeletePrefix removes every file or object below keyPrefix. A missing prefix is not an error.
func DeletePrefix(ctx context.Context, dataDir, keyPrefix string) error {
	local, err := localPath(dataDir, keyPrefix)
	if err != nil {
		return err
	}
	if !Enabled() {
		return os.RemoveAll(local)
	}
	keys, err := listKeys(ctx, keyPrefix+"/")
	if err != nil {
		return err
	}
	cli, bucket, err := client(ctx)
	if err != nil {
		return err
	}
	for len(keys) > 0 {
		n := min(len(keys), deleteBatch)
		ids := make([]types.ObjectIdentifier, n)
		for i, k := range keys[:n] {
			ids[i] = types.ObjectIdentifier{Key: aws.String(k)}
		}
		keys = keys[n:]
		cctx, cancel := context.WithTimeout(ctx, callTimeout)
		out, err := cli.DeleteObjects(cctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(bucket), Delete: &types.Delete{Objects: ids, Quiet: aws.Bool(true)},
		})
		cancel()
		if err != nil {
			return err
		}
		if len(out.Errors) > 0 {
			return fmt.Errorf("delete %s: %d objects failed, first %s: %s",
				keyPrefix, len(out.Errors), aws.ToString(out.Errors[0].Key), aws.ToString(out.Errors[0].Message))
		}
	}
	return nil
}

// localPath validates the key and returns dataDir/<key>. Keys are slash separated, relative and clean.
func localPath(dataDir, key string) (string, error) {
	if key == "" || path.IsAbs(key) || path.Clean(key) != key || key == ".." || strings.HasPrefix(key, "../") {
		return "", fmt.Errorf("invalid workspace key %q", key)
	}
	return filepath.Join(dataDir, filepath.FromSlash(key)), nil
}

func client(ctx context.Context) (*s3.Client, string, error) {
	bucket := awsenv.LakeBucket()
	if bucket == "" {
		return nil, "", fmt.Errorf("LAKE_BUCKET required for LAKE_BACKEND=s3")
	}
	cfg, err := awsenv.Load(ctx)
	if err != nil {
		return nil, "", err
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		if ep := awsenv.ResolveEndpoint(); ep != nil {
			o.BaseEndpoint = ep
			o.UsePathStyle = true
		}
	}), bucket, nil
}

func put(ctx context.Context, key string, body []byte) error {
	cli, bucket, err := client(ctx)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	_, err = cli.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket), Key: aws.String(key), Body: bytes.NewReader(body),
		ContentType: aws.String("application/octet-stream"),
	})
	return err
}

// listKeys returns all object keys that start with prefix (paginated).
func listKeys(ctx context.Context, prefix string) ([]string, error) {
	cli, bucket, err := client(ctx)
	if err != nil {
		return nil, err
	}
	var keys []string
	var token *string
	for {
		out, err := cli.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket: aws.String(bucket), Prefix: aws.String(prefix), ContinuationToken: token,
		})
		if err != nil {
			return nil, err
		}
		for _, o := range out.Contents {
			keys = append(keys, aws.ToString(o.Key))
		}
		if !aws.ToBool(out.IsTruncated) {
			return keys, nil
		}
		token = out.NextContinuationToken
	}
}

func wrapNotFound(key string, err error) error {
	var noKey *types.NoSuchKey
	var notFound *types.NotFound
	if errors.As(err, &noKey) || errors.As(err, &notFound) {
		return fmt.Errorf("%s: %w", key, fs.ErrNotExist)
	}
	return err
}
