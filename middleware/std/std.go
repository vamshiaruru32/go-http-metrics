// Package std is a helper package to get a standard `http.Handler` compatible middleware.
package std

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/slok/go-http-metrics/middleware"
)

type ChiReporter struct {
	r *http.Request
	w *responseWriterInterceptor
}

func (c *ChiReporter) Method() string { return c.r.Method }

func (c *ChiReporter) Context() context.Context { return c.r.Context() }

func (c *ChiReporter) URLPath() string {
	// get from chi context
	return chi.RouteContext(c.r.Context()).RoutePattern()
}

func (c *ChiReporter) StatusCode() int { return c.w.statusCode }

func (c *ChiReporter) BytesWritten() int64 { return int64(c.w.bytesWritten) }

// Handler returns an measuring standard http.Handler.
func Handler(handlerID string, m middleware.Middleware, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wi := &responseWriterInterceptor{
			statusCode:     http.StatusOK,
			ResponseWriter: w,
		}
		var reporter middleware.Reporter
		reporter = &stdReporter{
			w: wi,
			r: r,
		}
		if m.Config().UseChi {
			reporter = &ChiReporter{
				r: r,
				w: wi,
			}
		}
		m.Measure(handlerID, reporter, func() {
			h.ServeHTTP(wi, r)
		})
	})
}

// HandlerProvider is a helper method that returns a handler provider. This kind of
// provider is a defacto standard in some frameworks (e.g: Gorilla, Chi...).
func HandlerProvider(handlerID string, m middleware.Middleware) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return Handler(handlerID, m, next)
	}
}

type stdReporter struct {
	w *responseWriterInterceptor
	r *http.Request
}

func (s *stdReporter) Method() string { return s.r.Method }

func (s *stdReporter) Context() context.Context { return s.r.Context() }

func (s *stdReporter) URLPath() string { return s.r.URL.Path }

func (s *stdReporter) StatusCode() int { return s.w.statusCode }

func (s *stdReporter) BytesWritten() int64 { return int64(s.w.bytesWritten) }

// responseWriterInterceptor is a simple wrapper to intercept set data on a
// ResponseWriter.
type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (w *responseWriterInterceptor) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriterInterceptor) Write(p []byte) (int, error) {
	w.bytesWritten += len(p)
	return w.ResponseWriter.Write(p)
}

func (w *responseWriterInterceptor) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("type assertion failed http.ResponseWriter not a http.Hijacker")
	}
	return h.Hijack()
}

func (w *responseWriterInterceptor) Flush() {
	f, ok := w.ResponseWriter.(http.Flusher)
	if !ok {
		return
	}

	f.Flush()
}

// Check interface implementations.
var (
	_ http.ResponseWriter = &responseWriterInterceptor{}
	_ http.Hijacker       = &responseWriterInterceptor{}
	_ http.Flusher        = &responseWriterInterceptor{}
)
