package providers

import (
	"reflect"
	"testing"
)

func TestSecretObjectRoundTrip(t *testing.T) {
	body, err := EncodeSecret(map[string]string{"apiKey": "k1", "apiSecret": "s1"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeSecret(body)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, map[string]string{"apiKey": "k1", "apiSecret": "s1"}) {
		t.Fatalf("round trip: %v", got)
	}
	if empty, err := DecodeSecret("  "); err != nil || len(empty) != 0 {
		t.Fatalf("blank secret: %v %v", empty, err)
	}
	if _, err := DecodeSecret(`["not","an","object"]`); err == nil {
		t.Fatal("a secret that is not a JSON object must fail")
	}
}

func TestMergeSecretsKeepsStoredAndTrims(t *testing.T) {
	stored := map[string]string{"apiKey": "old", "apiSecret": "keep"}
	got := MergeSecrets(stored, map[string]string{"apiKey": "  new  "})
	want := map[string]string{"apiKey": "new", "apiSecret": "keep"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merge: %v want %v", got, want)
	}
	if stored["apiKey"] != "old" {
		t.Fatal("merge must not change its input")
	}
}

func TestConfiguredKeysSortedAndNonBlank(t *testing.T) {
	got := ConfiguredKeys(map[string]string{"b": "x", "a": "y", "blank": "   ", "empty": ""})
	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("configured keys: %v", got)
	}
	if got := ConfiguredKeys(nil); got == nil || len(got) != 0 {
		t.Fatalf("no secrets must be an empty, non-nil list: %v", got)
	}
}
