package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func TestJSONRequestRoundTrip(t *testing.T) {
	ev := events.APIGatewayV2HTTPRequest{
		RawPath:        "/tradingcopilot.v1.ExperimentService/ListExperiments",
		RawQueryString: "page=2",
		Headers:        map[string]string{"content-type": "application/json", "connect-protocol-version": "1"},
		Cookies:        []string{"a=1", "b=2"},
		Body:           `{"limit":5}`,
		RequestContext: events.APIGatewayV2HTTPRequestContext{HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{Method: http.MethodPost}},
	}
	req, err := toHTTPRequest(context.Background(), ev)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != http.MethodPost || req.URL.Path != ev.RawPath || req.URL.RawQuery != "page=2" {
		t.Fatalf("method/url = %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
	}
	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q", got)
	}
	if got := req.Header.Get("Cookie"); got != "a=1; b=2" {
		t.Fatalf("cookie = %q", got)
	}
	body, _ := io.ReadAll(req.Body)
	if string(body) != ev.Body || req.ContentLength != int64(len(ev.Body)) {
		t.Fatalf("body = %q (len %d)", body, req.ContentLength)
	}
}

func TestBase64RequestBodyIsDecoded(t *testing.T) {
	payload := []byte{0x0a, 0x03, 'f', 'o', 'o'}
	ev := events.APIGatewayV2HTTPRequest{
		RawPath:         "/svc/Method",
		Headers:         map[string]string{"content-type": "application/proto"},
		Body:            base64.StdEncoding.EncodeToString(payload),
		IsBase64Encoded: true,
		RequestContext:  events.APIGatewayV2HTTPRequestContext{HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{Method: http.MethodPost}},
	}
	req, err := toHTTPRequest(context.Background(), ev)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(req.Body)
	if !bytes.Equal(body, payload) {
		t.Fatalf("body = %x, want %x", body, payload)
	}
}

func TestInvalidBase64RequestBodyFails(t *testing.T) {
	ev := events.APIGatewayV2HTTPRequest{Body: "!!!", IsBase64Encoded: true}
	if _, err := toHTTPRequest(context.Background(), ev); err == nil {
		t.Fatal("expected error for invalid base64 body")
	}
}

func TestProtoResponseIsBase64Encoded(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Type", "application/proto")
	rec.WriteHeader(http.StatusOK)
	rec.Write([]byte{0x08, 0x96, 0x01})

	res := toAPIResponse(rec)
	if !res.IsBase64Encoded {
		t.Fatal("protobuf response must be base64-encoded")
	}
	decoded, err := base64.StdEncoding.DecodeString(res.Body)
	if err != nil || !bytes.Equal(decoded, []byte{0x08, 0x96, 0x01}) {
		t.Fatalf("decoded body = %x, err %v", decoded, err)
	}
}

func TestGzipResponseIsBase64Encoded(t *testing.T) {
	var zbuf bytes.Buffer
	zw := gzip.NewWriter(&zbuf)
	zw.Write([]byte(`{"ok":true}`))
	zw.Close()

	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Type", "application/json")
	rec.Header().Set("Content-Encoding", "gzip")
	rec.Write(zbuf.Bytes())

	res := toAPIResponse(rec)
	if !res.IsBase64Encoded {
		t.Fatal("gzip response must be base64-encoded even with a JSON content type")
	}
	decoded, _ := base64.StdEncoding.DecodeString(res.Body)
	if !bytes.Equal(decoded, zbuf.Bytes()) {
		t.Fatal("base64 body does not decode to the gzip payload")
	}
}

func TestTextResponseIsNotEncoded(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Type", "application/json")
	rec.Header().Add("X-Multi", "a")
	rec.Header().Add("X-Multi", "b")
	rec.WriteHeader(http.StatusNotFound)
	rec.Write([]byte(`{"code":"not_found"}`))

	res := toAPIResponse(rec)
	if res.IsBase64Encoded {
		t.Fatal("JSON response must not be base64-encoded")
	}
	if res.StatusCode != http.StatusNotFound || res.Body != `{"code":"not_found"}` {
		t.Fatalf("status/body = %d %q", res.StatusCode, res.Body)
	}
	if got := res.Headers["X-Multi"]; got != "a, b" {
		t.Fatalf("multi-value header = %q", got)
	}
}
