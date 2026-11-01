package experimentapi

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGzipResponseDecodesToIdenticalBody(t *testing.T) {
	mux := http.NewServeMux()
	Mount(mux, NewServer(nil, Hooks{}))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	get := func(acceptEncoding string) (*http.Response, []byte) {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/experiments/strategies", nil)
		if acceptEncoding != "" {
			req.Header.Set("Accept-Encoding", acceptEncoding)
		}
		// Transport must not decode for us: that would hide what the server sent.
		tr := &http.Transport{DisableCompression: true}
		resp, err := (&http.Client{Transport: tr}).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp, body
	}

	plain, plainBody := get("")
	if plain.Header.Get("Content-Encoding") != "" {
		t.Fatalf("no Accept-Encoding: Content-Encoding %q", plain.Header.Get("Content-Encoding"))
	}
	if plain.Header.Get("Vary") != "Accept-Encoding" {
		t.Fatalf("Vary %q", plain.Header.Get("Vary"))
	}

	zipped, zippedBody := get("gzip, deflate")
	if zipped.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("Content-Encoding %q", zipped.Header.Get("Content-Encoding"))
	}
	if zipped.Header.Get("Content-Type") != plain.Header.Get("Content-Type") {
		t.Fatalf("Content-Type changed: %q vs %q", zipped.Header.Get("Content-Type"), plain.Header.Get("Content-Type"))
	}
	gr, err := gzip.NewReader(bytes.NewReader(zippedBody))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(gr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, plainBody) {
		t.Fatalf("decoded body differs:\n%s\n%s", decoded, plainBody)
	}
	if zipped.StatusCode != plain.StatusCode {
		t.Fatalf("status %d vs %d", zipped.StatusCode, plain.StatusCode)
	}
}

func TestAcceptsGzip(t *testing.T) {
	cases := map[string]bool{
		"":                    false,
		"identity":            false,
		"gzip":                true,
		"deflate, gzip;q=0.5": true,
		"gzip;q=0":            false,
		"gzip; q=0.0":         false,
		"gzip;q=0.1":          true,
		"br, GZIP":            true,
		"gzipper":             false,
	}
	for in, want := range cases {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if in != "" {
			r.Header.Set("Accept-Encoding", in)
		}
		if got := acceptsGzip(r); got != want {
			t.Errorf("Accept-Encoding %q: got %v want %v", in, got, want)
		}
	}
}
