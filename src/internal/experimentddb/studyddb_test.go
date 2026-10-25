package experimentddb

import "testing"

func TestGSI1SKNewestFirst(t *testing.T) {
	newer := gsi1SK(1_700_000_000_000, "aaa")
	older := gsi1SK(1_600_000_000_000, "bbb")
	if newer >= older {
		t.Fatalf("newer experiment should sort before older on gsi1: newer=%q older=%q", newer, older)
	}
	if newer != "8299999999999#aaa" {
		t.Fatalf("unexpected gsi1sk: %q", newer)
	}
}

func TestStudyKeys(t *testing.T) {
	if experimentPK("id-1") != "EXPERIMENT#id-1" {
		t.Fatal(experimentPK("id-1"))
	}
	if experimentMetaSK() != "META" {
		t.Fatal(experimentMetaSK())
	}
	if instanceSK("i1") != "INST#i1" {
		t.Fatal(instanceSK("i1"))
	}
}
