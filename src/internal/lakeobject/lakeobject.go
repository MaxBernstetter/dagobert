package lakeobject

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/MaxBernstetter/dagobert/src/internal/awsenv"
	"github.com/MaxBernstetter/dagobert/src/shared"
)

// ObjectKey returns the S3 object key for an absolute lake path under dataDir.
func ObjectKey(dataDir, absPath string) (string, error) {
	dataDir = filepath.Clean(dataDir)
	abs, err := filepath.Abs(absPath)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(dataDir, abs)
	if err != nil {
		return "", err
	}
	key := filepath.ToSlash(rel)
	if !strings.HasPrefix(key, shared.DataParquetSubdir+"/") {
		return "", fmt.Errorf("lake object key must start with %s/: %q", shared.DataParquetSubdir, key)
	}
	return key, nil
}

func dataDirFromEnv() string {
	d := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if d == "" {
		return "/data"
	}
	return d
}

// FileHandle is a local or downloaded parquet file (implements io.ReaderAt via *os.File).
type FileHandle struct {
	*os.File
	remove string
}

func (h *FileHandle) Close() error {
	err := h.File.Close()
	if h.remove != "" {
		os.Remove(h.remove)
	}
	return err
}

// OpenAbs opens a parquet file using DATA_DIR for S3 key resolution.
func OpenAbs(ctx context.Context, absPath string) (*FileHandle, int64, error) {
	return Open(ctx, dataDirFromEnv(), absPath)
}

// Open reads a local lake file or downloads from S3 when LAKE_BACKEND=s3.
func Open(ctx context.Context, dataDir, absPath string) (*FileHandle, int64, error) {
	if awsenv.LakeBackend() != "s3" {
		f, err := os.Open(absPath)
		if err != nil {
			return nil, 0, err
		}
		st, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, 0, err
		}
		return &FileHandle{File: f}, st.Size(), nil
	}
	bucket := awsenv.LakeBucket()
	if bucket == "" {
		return nil, 0, fmt.Errorf("LAKE_BUCKET required for LAKE_BACKEND=s3")
	}
	key, err := ObjectKey(dataDir, absPath)
	if err != nil {
		return nil, 0, err
	}
	cfg, err := awsenv.Load(ctx)
	if err != nil {
		return nil, 0, err
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if ep := awsenv.ResolveEndpoint(); ep != nil {
			o.BaseEndpoint = ep
			o.UsePathStyle = true
		}
	})
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, 0, err
	}
	tmp, err := os.CreateTemp(os.TempDir(), "lake-*.parquet")
	if err != nil {
		out.Body.Close()
		return nil, 0, err
	}
	if _, err := io.Copy(tmp, out.Body); err != nil {
		out.Body.Close()
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, 0, err
	}
	out.Body.Close()
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, 0, err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, 0, err
	}
	st, err := tmp.Stat()
	if err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, 0, err
	}
	return &FileHandle{File: tmp, remove: tmp.Name()}, st.Size(), nil
}

// Exists reports whether the lake object is present locally or in S3.
func Exists(ctx context.Context, dataDir, absPath string) (bool, error) {
	if awsenv.LakeBackend() != "s3" {
		_, err := os.Stat(absPath)
		if err == nil {
			return true, nil
		}
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	bucket := awsenv.LakeBucket()
	if bucket == "" {
		return false, fmt.Errorf("LAKE_BUCKET required for LAKE_BACKEND=s3")
	}
	key, err := ObjectKey(dataDir, absPath)
	if err != nil {
		return false, err
	}
	cfg, err := awsenv.Load(ctx)
	if err != nil {
		return false, err
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if ep := awsenv.ResolveEndpoint(); ep != nil {
			o.BaseEndpoint = ep
			o.UsePathStyle = true
		}
	})
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_, err = client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var nsk *types.NotFound
		if _, ok := err.(*types.NotFound); ok || strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "404") {
			return false, nil
		}
		_ = nsk
		return false, err
	}
	return true, nil
}

// PutLocalFile uploads a local parquet file to the lake bucket when dual-write is enabled.
func PutLocalFile(ctx context.Context, dataDir, absPath string) error {
	if !awsenv.LakeDualWrite() {
		return nil
	}
	bucket := awsenv.LakeBucket()
	if bucket == "" {
		return nil
	}
	key, err := ObjectKey(dataDir, absPath)
	if err != nil {
		return err
	}
	body, err := os.ReadFile(absPath)
	if err != nil {
		return err
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
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String("application/octet-stream"),
	})
	return err
}

// ListCandleParquetAbsPaths lists candles.parquet paths for a market (fs or s3).
func ListCandleParquetAbsPaths(ctx context.Context, dataDir, provider, base, quote string) ([]string, error) {
	if awsenv.LakeBackend() != "s3" {
		return shared.ListCandleParquetPaths(dataDir, provider, base, quote)
	}
	m, err := shared.NormalizeMarket(base, quote)
	if err != nil {
		return nil, err
	}
	keyPrefix, err := ObjectKey(dataDir, filepath.Join(dataDir, shared.DataParquetSubdir,
		"provider="+provider, "base="+m.Base, "quote="+m.Quote))
	if err != nil {
		return nil, err
	}
	keyPrefix += "/"
	bucket := awsenv.LakeBucket()
	if bucket == "" {
		return nil, fmt.Errorf("LAKE_BUCKET required for LAKE_BACKEND=s3")
	}
	cfg, err := awsenv.Load(ctx)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if ep := awsenv.ResolveEndpoint(); ep != nil {
			o.BaseEndpoint = ep
			o.UsePathStyle = true
		}
	})
	var keys []string
	var token *string
	for {
		out, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(bucket),
			Prefix:            aws.String(keyPrefix),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, err
		}
		for _, obj := range out.Contents {
			if obj.Key == nil {
				continue
			}
			if strings.HasSuffix(*obj.Key, "/"+shared.CandlesParquetFile) {
				keys = append(keys, *obj.Key)
			}
		}
		if !aws.ToBool(out.IsTruncated) {
			break
		}
		token = out.NextContinuationToken
	}
	outPaths := make([]string, 0, len(keys))
	for _, key := range keys {
		outPaths = append(outPaths, filepath.Join(dataDir, filepath.FromSlash(key)))
	}
	return outPaths, nil
}

// ListCandleParquetAbsPathsInRange lists existing candle parquet files overlapping [fromMs, toMs).
func ListCandleParquetAbsPathsInRange(ctx context.Context, dataDir, provider, base, quote string, fromMs, toMs int64) ([]string, error) {
	if awsenv.LakeBackend() != "s3" {
		return shared.ListCandleParquetPathsInRange(dataDir, provider, base, quote, fromMs, toMs)
	}
	years := shared.YearsSpanning(fromMs, toMs)
	if len(years) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(years))
	for _, y := range years {
		path, err := shared.ParquetPath(dataDir, provider, base, quote, y)
		if err != nil {
			return nil, err
		}
		ok, err := Exists(ctx, dataDir, path)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, path)
		}
	}
	return out, nil
}
