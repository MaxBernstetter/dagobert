package experimentapi

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1/tradingcopilotv1connect"
)

// ListStrategies needs no database, so the transport can be tested without one.
func strategiesServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	Mount(mux, NewServer(nil, Hooks{}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestListStrategiesProtobufAndJSON(t *testing.T) {
	srv := strategiesServer(t)

	client := tradingcopilotv1connect.NewExperimentServiceClient(srv.Client(), srv.URL)
	resp, err := client.ListStrategies(t.Context(), connect.NewRequest(&v1.ListStrategiesRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Msg.Strategies) != len(experimentReadyStrategies) {
		t.Fatalf("strategies %d want %d", len(resp.Msg.Strategies), len(experimentReadyStrategies))
	}

	post, err := http.Post(srv.URL+tradingcopilotv1connect.ExperimentServiceListStrategiesProcedure, "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer post.Body.Close()
	var body struct {
		Strategies []struct {
			ID string `json:"id"`
		} `json:"strategies"`
	}
	if err := json.NewDecoder(post.Body).Decode(&body); err != nil || post.StatusCode != http.StatusOK {
		t.Fatalf("JSON call: %d %v", post.StatusCode, err)
	}
	if len(body.Strategies) == 0 || body.Strategies[0].ID != "ma-cross" {
		t.Fatalf("JSON strategies: %+v", body.Strategies)
	}
}

func TestResponsesAreGzipped(t *testing.T) {
	srv := strategiesServer(t)
	call := func(acceptEncoding string) (*http.Response, []byte) {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+tradingcopilotv1connect.ExperimentServiceListStrategiesProcedure, bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Encoding", acceptEncoding)
		// The transport must not decode for us: that would hide what the server sent.
		resp, err := (&http.Client{Transport: &http.Transport{DisableCompression: true}}).Do(req)
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

	plain, plainBody := call("identity")
	if plain.Header.Get("Content-Encoding") != "" {
		t.Fatalf("identity: Content-Encoding %q", plain.Header.Get("Content-Encoding"))
	}
	zipped, zippedBody := call("gzip")
	if zipped.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("gzip: Content-Encoding %q", zipped.Header.Get("Content-Encoding"))
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
}
