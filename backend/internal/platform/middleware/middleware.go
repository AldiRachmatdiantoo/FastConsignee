// Package middleware berisi middleware HTTP lintas fitur.
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/api"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/apperr"
)

// Middleware membungkus handler.
type Middleware func(http.Handler) http.Handler

// Chain menerapkan middleware dari luar ke dalam.
func Chain(h http.Handler, m ...Middleware) http.Handler {
	for i := len(m) - 1; i >= 0; i-- {
		h = m[i](h)
	}
	return h
}

var safeRequestID = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

// RequestID memberi tiap permintaan ID dan menuliskannya di header jawaban.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if !safeRequestID.MatchString(id) {
			id = newID()
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(api.WithRequestID(r.Context(), id)))
	})
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type recorder struct {
	http.ResponseWriter
	status int
}

func (r *recorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Recover mengubah panic menjadi 500 tanpa membocorkan detailnya.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				slog.Error("panic", "value", v, "stack", string(debug.Stack()), "request_id", api.RequestID(r.Context()))
				api.Error(w, r, fmt.Errorf("panic: %v", v))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Logger mencatat tiap permintaan tanpa body dan tanpa header Authorization.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &recorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if r.URL.Path == "/healthz" {
			return
		}
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", api.RequestID(r.Context()),
		)
	})
}

// SecurityHeaders menambahkan header pengaman dasar.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

const solanaPayPrefix = "/api/v1/solana-pay/"

// CORS membuka endpoint Solana Pay untuk semua origin, dan endpoint lain hanya untuk frontend.
func CORS(frontendOrigin string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			origin := r.Header.Get("Origin")
			preflight := r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
			switch {
			case strings.HasPrefix(r.URL.Path, solanaPayPrefix):
				h.Set("Access-Control-Allow-Origin", "*")
				h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Content-Type, Accept, Accept-Encoding")
			case origin != "" && origin == frontendOrigin:
				h.Add("Vary", "Origin")
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key")
				h.Set("Access-Control-Expose-Headers", "X-Request-Id, Idempotency-Replayed, Location, Retry-After, RateLimit-Limit, RateLimit-Remaining, RateLimit-Reset")
				h.Set("Access-Control-Max-Age", "600")
			}
			if preflight {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIP membaca IP asli. hops adalah jumlah proxy tepercaya di depan API, jadi nilai
// X-Forwarded-For yang dikirim klien tidak bisa memalsukan IP.
func ClientIP(r *http.Request, hops int) string {
	if hops > 0 {
		parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
		if i := len(parts) - hops; i >= 0 && strings.TrimSpace(parts[i]) != "" {
			return strings.TrimSpace(parts[i])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// RateHeaders menulis header RateLimit-* dan memeriksa hasil limiter.
func RateHeaders(w http.ResponseWriter, d Decision) {
	h := w.Header()
	h.Set("RateLimit-Limit", strconv.Itoa(d.Limit))
	h.Set("RateLimit-Remaining", strconv.Itoa(d.Remaining))
	h.Set("RateLimit-Reset", strconv.Itoa(d.ResetSeconds))
}

// Reject menulis jawaban 429 dengan bentuk error yang diberikan.
func Reject(w http.ResponseWriter, r *http.Request, d Decision) {
	w.Header().Set("Retry-After", strconv.Itoa(max(d.RetryAfterSeconds, 1)))
	api.Error(w, r, apperr.New(apperr.RateLimited, "Terlalu banyak permintaan. Coba lagi sebentar lagi."))
}
