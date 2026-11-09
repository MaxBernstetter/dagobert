package workspace

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestFsRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := WriteBytes(ctx, dir, "experiments/e/.cache/kpi/BTC.json", []byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := WriteBytes(ctx, dir, "experiments/e/.cache/meta.json", []byte("b")); err != nil {
		t.Fatal(err)
	}
	got, err := ReadBytes(ctx, dir, "experiments/e/.cache/meta.json")
	if err != nil || string(got) != "b" {
		t.Fatalf("read %q: %v", got, err)
	}
	names, err := List(ctx, dir, "experiments/e/.cache")
	if err != nil || len(names) != 1 || names[0] != "meta.json" {
		t.Fatalf("list %v: %v", names, err)
	}
	if err := DeletePrefix(ctx, dir, "experiments/e/.cache"); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadBytes(ctx, dir, "experiments/e/.cache/meta.json"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("want ErrNotExist after delete, got %v", err)
	}
	if names, err := List(ctx, dir, "experiments/e/.cache"); err != nil || len(names) != 0 {
		t.Fatalf("missing prefix lists %v: %v", names, err)
	}
	if err := DeletePrefix(ctx, dir, "experiments/missing"); err != nil {
		t.Fatalf("deleting a missing prefix: %v", err)
	}
}

func TestPublishDirFsKeepsFiles(t *testing.T) {
	dir := t.TempDir()
	shard := filepath.Join(dir, "experiments/e/shards/s")
	if err := os.MkdirAll(shard, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shard, "x.parquet"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PublishDir(context.Background(), dir, "experiments/e/shards/s"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(shard, "x.parquet")); err != nil {
		t.Fatalf("fs mode must keep the shard: %v", err)
	}
}

func TestInvalidKeys(t *testing.T) {
	for _, key := range []string{"", "/abs", "../x", "a/../b", "a//b", "a/"} {
		if _, _, err := Open(context.Background(), t.TempDir(), key); err == nil {
			t.Errorf("key %q accepted", key)
		}
	}
}
