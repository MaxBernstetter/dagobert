package backtest

import "testing"

func TestChunkStrings(t *testing.T) {
	if ChunkStrings(nil, 64) != nil {
		t.Fatal("nil ids")
	}
	all := []string{"a", "b", "c", "d", "e"}
	chunks := ChunkStrings(all, 2)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if len(chunks[0]) != 2 || chunks[2][0] != "e" {
		t.Fatalf("unexpected chunks: %#v", chunks)
	}
	one := ChunkStrings(all, 0)
	if len(one) != 1 || len(one[0]) != 5 {
		t.Fatalf("size<1: %#v", one)
	}
}
