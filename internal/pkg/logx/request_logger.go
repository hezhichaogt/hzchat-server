/*
Package logx provides a structured logging wrapper based on zerolog.

This file contains middleware functions for HTTP routing, used to log request lifecycle information
such as URI, method, response status, and latency. It also implements an IP address anonymization
feature to enhance user privacy.
*/
package logx

import (
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// anonymizeIP anonymizes the given IP address string.
// For IPv4, it zeros out the last octet; for IPv6, it compresses the latter half to "::".
// This preserves approximate geolocation while enhancing user privacy.
func anonymizeIP(ipStr string) string {
	host, _, err := net.SplitHostPort(ipStr)
	if err == nil {
		ipStr = host
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "unknown_ip"
	}

	if ip.IsLoopback() {
		return "127.0.0.1"
	}

	if v4 := ip.To4(); v4 != nil {
		return v4[:3].String() + ".0"
	}

	if v6 := ip.To16(); v6 != nil {
		return v6[:8].String() + "::"
	}

	return ipStr
}

// RequestLogger returns an HTTP middleware function that logs detailed information about the HTTP request.
// It creates a new logger instance for each request and injects it into the request context.
func RequestLogger() func(next http.Handler) http.Handler {
	baseLogger := Logger()

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			requestID := middleware.GetReqID(r.Context())

			anonIP := anonymizeIP(r.RemoteAddr)

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			logger := baseLogger.With().
				Str("component", "http").
				Str("request_id", requestID).
				Str("remote_ip", anonIP).
				Str("request_method", r.Method).
				Str("request_uri", r.RequestURI).
				Logger()

			r = r.WithContext(logger.WithContext(r.Context()))

			t1 := time.Now()
			next.ServeHTTP(ww, r)

			status := ww.Status()

			logEvent := logger.Info()
			if status >= 500 {
				logEvent = logger.Error()
			} else if status >= 400 {
				logEvent = logger.Warn()
			}

			logEvent.
				Int("status", status).
				Int("bytes", ww.BytesWritten()).
				Dur("latency", time.Since(t1)).
				Msg("Request completed")
		}

		return http.HandlerFunc(fn)
	}
}
