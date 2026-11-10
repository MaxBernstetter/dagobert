package main

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

// toHTTPRequest converts an API Gateway HTTP API (payload format 2.0) event into a net/http request.
func toHTTPRequest(ctx context.Context, ev events.APIGatewayV2HTTPRequest) (*http.Request, error) {
	body := []byte(ev.Body)
	if ev.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(ev.Body)
		if err != nil {
			return nil, err
		}
		body = decoded
	}

	target := ev.RawPath
	if target == "" {
		target = "/"
	}
	if ev.RawQueryString != "" {
		target += "?" + ev.RawQueryString
	}
	req, err := http.NewRequestWithContext(ctx, ev.RequestContext.HTTP.Method, target, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	for k, v := range ev.Headers {
		req.Header.Set(k, v)
	}
	if len(ev.Cookies) > 0 {
		req.Header.Set("Cookie", strings.Join(ev.Cookies, "; "))
	}
	req.ContentLength = int64(len(body))
	return req, nil
}

// toAPIResponse converts a recorded handler response into an API Gateway HTTP API response. Multi-valued
// headers are joined with ", ", except Set-Cookie, which API Gateway takes from the cookies list. Bodies
// that are not text (protobuf, or any compressed encoding) are base64-encoded so they survive the JSON envelope.
func toAPIResponse(rec *httptest.ResponseRecorder) events.APIGatewayV2HTTPResponse {
	res := rec.Result()
	defer res.Body.Close()

	headers := make(map[string]string, len(res.Header))
	var cookies []string
	for k, vs := range res.Header {
		if k == "Set-Cookie" {
			cookies = vs
			continue
		}
		headers[k] = strings.Join(vs, ", ")
	}

	body := rec.Body.Bytes()
	out := events.APIGatewayV2HTTPResponse{
		StatusCode: res.StatusCode,
		Headers:    headers,
		Cookies:    cookies,
	}
	if needsBase64(res.Header) {
		out.Body = base64.StdEncoding.EncodeToString(body)
		out.IsBase64Encoded = true
	} else {
		out.Body = string(body)
	}
	return out
}

// needsBase64 reports whether a response body must be base64-encoded: any Content-Encoding (gzip), or a
// Content-Type that is not JSON or text.
func needsBase64(h http.Header) bool {
	if h.Get("Content-Encoding") != "" {
		return true
	}
	ct := h.Get("Content-Type")
	if ct == "" {
		return false
	}
	return !(strings.HasPrefix(ct, "text/") || strings.HasPrefix(ct, "application/json") || strings.Contains(ct, "+json"))
}
