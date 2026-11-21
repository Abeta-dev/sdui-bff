// Package handler implements the Gin HTTP transport layer for the Abeta SDUI Domain BFF,
// providing Brotli/gzip compression, SHA-256 ETag caching, OpenAPI 3.1 docs, and vertical endpoints.
package handler

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/gin-gonic/gin"
)

const defaultCacheControl = "public, max-age=30"

type bufferedResponseWriter struct {
	gin.ResponseWriter
	buf         bytes.Buffer
	status      int
	wroteHeader bool
}

func newBufferedResponseWriter(w gin.ResponseWriter) *bufferedResponseWriter {
	return &bufferedResponseWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
	}
}

func (w *bufferedResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.status = code
	w.wroteHeader = true
}

func (w *bufferedResponseWriter) WriteHeaderNow() {
	if !w.wroteHeader {
		w.wroteHeader = true
		w.status = http.StatusOK
	}
}

func (w *bufferedResponseWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.buf.Write(data)
}

func (w *bufferedResponseWriter) WriteString(s string) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.buf.WriteString(s)
}

func (w *bufferedResponseWriter) Status() int {
	return w.status
}

func (w *bufferedResponseWriter) Size() int {
	return w.buf.Len()
}

func (w *bufferedResponseWriter) Written() bool {
	return w.wroteHeader
}

// CompressionMiddleware negotiates Brotli ("br") or gzip ("gzip") response compression
// based on Accept-Encoding and appends Vary: Accept-Encoding.
func CompressionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodHead {
			c.Next()
			return
		}

		appendVary(c.Writer.Header(), "Accept-Encoding")

		enc := negotiateEncoding(c.GetHeader("Accept-Encoding"))
		if enc == "" {
			c.Next()
			return
		}

		orig := c.Writer
		bw := newBufferedResponseWriter(orig)
		c.Writer = bw
		completed := false
		defer func() {
			if !completed {
				c.Writer = orig
			}
		}()

		c.Next()
		completed = true
		c.Writer = orig

		status := bw.Status()
		body := bw.buf.Bytes()

		if !bodyAllowedForStatus(status) || len(body) == 0 || orig.Header().Get("Content-Encoding") != "" {
			if status == http.StatusNotModified {
				weakenETag(orig.Header())
			}
			if bw.wroteHeader || len(body) > 0 {
				orig.WriteHeader(status)
			}
			if len(body) > 0 && bodyAllowedForStatus(status) {
				_, _ = orig.Write(body)
			}
			return
		}

		compressed, err := compressBytes(enc, body)
		if err != nil {
			if bw.wroteHeader || len(body) > 0 {
				orig.WriteHeader(status)
			}
			_, _ = orig.Write(body)
			return
		}

		orig.Header().Set("Content-Encoding", enc)
		orig.Header().Del("Content-Length")
		weakenETag(orig.Header())
		orig.WriteHeader(status)
		_, _ = orig.Write(compressed)
	}
}

// ETagMiddleware computes a SHA-256 ETag over successful GET JSON responses,
// sets Cache-Control: public, max-age=30, and honors If-None-Match with 304 Not Modified.
func ETagMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}

		orig := c.Writer
		bw := newBufferedResponseWriter(orig)
		c.Writer = bw
		completed := false
		defer func() {
			if !completed {
				c.Writer = orig
			}
		}()

		c.Next()
		completed = true
		c.Writer = orig

		status := bw.Status()
		body := bw.buf.Bytes()
		contentType := strings.ToLower(orig.Header().Get("Content-Type"))

		if status < 200 || status >= 300 || len(body) == 0 || !strings.Contains(contentType, "application/json") {
			if bw.wroteHeader || len(body) > 0 {
				orig.WriteHeader(status)
			}
			if len(body) > 0 && bodyAllowedForStatus(status) {
				_, _ = orig.Write(body)
			}
			return
		}

		sum := sha256.Sum256(body)
		etag := fmt.Sprintf(`"%x"`, sum)

		h := orig.Header()
		h.Set("ETag", etag)
		h.Set("Cache-Control", defaultCacheControl)
		h.Del("Pragma")

		if ifNoneMatchMatches(c.GetHeader("If-None-Match"), etag) {
			h.Del("Content-Length")
			h.Del("Content-Type")
			orig.WriteHeader(http.StatusNotModified)
			return
		}

		orig.WriteHeader(status)
		_, _ = orig.Write(body)
	}
}

func compressBytes(encoding string, raw []byte) ([]byte, error) {
	var out bytes.Buffer
	var zw io.WriteCloser
	switch encoding {
	case "br":
		zw = brotli.NewWriterLevel(&out, brotli.DefaultCompression)
	case "gzip":
		zw = gzip.NewWriter(&out)
	default:
		return raw, nil
	}
	if _, err := zw.Write(raw); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func bodyAllowedForStatus(status int) bool {
	switch {
	case status >= 100 && status <= 199:
		return false
	case status == http.StatusNoContent:
		return false
	case status == http.StatusNotModified:
		return false
	default:
		return true
	}
}

func weakenETag(h http.Header) {
	if etag := h.Get("ETag"); etag != "" && !strings.HasPrefix(etag, "W/") {
		h.Set("ETag", "W/"+etag)
	}
}

func ifNoneMatchMatches(headerVal, currentETag string) bool {
	trimmed := strings.TrimSpace(headerVal)
	if trimmed == "" {
		return false
	}
	if trimmed == "*" {
		return true
	}

	normCurrent := strings.TrimPrefix(currentETag, "W/")
	for _, part := range strings.Split(trimmed, ",") {
		cand := strings.TrimSpace(part)
		if cand == "*" {
			return true
		}
		if cand == "" {
			continue
		}
		if strings.TrimPrefix(cand, "W/") == normCurrent {
			return true
		}
	}
	return false
}

func appendVary(h http.Header, value string) {
	existing := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)
	for _, raw := range h.Values("Vary") {
		for _, tok := range strings.Split(raw, ",") {
			trimmed := strings.TrimSpace(tok)
			if trimmed == "" {
				continue
			}
			lower := strings.ToLower(trimmed)
			if _, ok := seen[lower]; !ok {
				seen[lower] = struct{}{}
				existing = append(existing, trimmed)
			}
		}
	}
	if _, ok := seen[strings.ToLower(value)]; !ok {
		existing = append(existing, value)
	}
	h.Set("Vary", strings.Join(existing, ", "))
}

func negotiateEncoding(acceptEncoding string) string {
	if strings.TrimSpace(acceptEncoding) == "" {
		return ""
	}
	brQ := -1.0
	gzipQ := -1.0
	wildcardQ := -1.0

	for _, part := range strings.Split(acceptEncoding, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		segments := strings.Split(part, ";")
		coding := strings.ToLower(strings.TrimSpace(segments[0]))
		q := 1.0
		valid := true
		for _, param := range segments[1:] {
			param = strings.TrimSpace(param)
			if len(param) >= 2 && strings.EqualFold(param[:2], "q=") {
				parsed, err := strconv.ParseFloat(strings.TrimSpace(param[2:]), 64)
				if err != nil || parsed < 0 || parsed > 1 {
					valid = false
					break
				}
				q = parsed
			}
		}
		if !valid {
			continue
		}
		switch coding {
		case "br":
			if q > brQ {
				brQ = q
			}
		case "gzip":
			if q > gzipQ {
				gzipQ = q
			}
		case "*":
			if q > wildcardQ {
				wildcardQ = q
			}
		}
	}

	if brQ < 0 && wildcardQ >= 0 {
		brQ = wildcardQ
	}
	if gzipQ < 0 && wildcardQ >= 0 {
		gzipQ = wildcardQ
	}
	if brQ <= 0 && gzipQ <= 0 {
		return ""
	}
	if brQ >= gzipQ {
		return "br"
	}
	return "gzip"
}
