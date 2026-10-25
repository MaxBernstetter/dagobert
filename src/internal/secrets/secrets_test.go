package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets")
	body := "export BYBIT_API_KEY=abc\nBYBIT_API_SECRET=\"sec\"\n# comment\nBYBIT_BASE_URL=https://example.test\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	out := Env{}
	if err := mergeFile(out, path); err != nil {
		t.Fatal(err)
	}
	if out["BYBIT_API_KEY"] != "abc" || out["BYBIT_API_SECRET"] != "sec" {
		t.Fatalf("got %#v", out)
	}
	if out["BYBIT_BASE_URL"] != "https://example.test" {
		t.Fatalf("base url: %q", out["BYBIT_BASE_URL"])
	}
}

func TestWriteEnvFile(t *testing.T) {
	path, err := WriteEnvFile(Env{
		"BYBIT_API_KEY":    "k",
		"BYBIT_API_SECRET": "s",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) == "" {
		t.Fatal("empty env file")
	}
}
