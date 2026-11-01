package experimentapi

import (
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// gzipPool reuses compressors: the chart series and the activity rows are megabytes per request.
var gzipPool = sync.Pool{New: func() any {
	w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
	return w
}}

// acceptsGzip reports whether the request accepts a gzip coded response: a gzip token with q > 0.
func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if !strings.EqualFold(strings.TrimSpace(name), "gzip") {
			continue
		}
		q := 1.0
		for _, p := range strings.Split(params, ";") {
			k, v, ok := strings.Cut(strings.TrimSpace(p), "=")
			if ok && strings.EqualFold(strings.TrimSpace(k), "q") {
				if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
					q = f
				}
			}
		}
		return q > 0
	}
	return false
}

// gzipHandler compresses the response when the client accepts gzip and leaves it unchanged otherwise.
// Vary: Accept-Encoding is set on every response so that caches keep the two variants apart.
func gzipHandler(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")
		if !acceptsGzip(r) {
			h(w, r)
			return
		}
		gz := gzipPool.Get().(*gzip.Writer)
		gz.Reset(w)
		defer func() {
			_ = gz.Close()
			gzipPool.Put(gz)
		}()
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")
		h(&gzipResponse{ResponseWriter: w, gz: gz}, r)
	})
}

// gzipResponse writes the body through the compressor. Status and headers pass through unchanged.
type gzipResponse struct {
	http.ResponseWriter
	gz *gzip.Writer
}

func (g *gzipResponse) Write(b []byte) (int, error) {
	return g.gz.Write(b)
}
