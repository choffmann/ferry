package httpapi

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/choffmann/ferry/internal/chaos"
	"github.com/choffmann/ferry/internal/obs"
)

type requestIDKey struct{}

func requestIDFrom(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}

// crypto/rand and math/rand/v2 are both called rand, hence the alias. The request
// id wants the cryptographic one, the error-rate draw does not.
func newRequestID() string {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b[:])
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

func recoverer(l *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					l.Error("panic im Handler", "panic", v, "path", r.URL.Path,
						"request_id", requestIDFrom(r.Context()))
					writeError(w, r, http.StatusInternalServerError, "interner Fehler")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func logging(l *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := requestIDFrom(r.Context())
			reqLogger := l.With("request_id", id)
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			start := time.Now()
			next.ServeHTTP(rec, r.WithContext(obs.WithLogger(r.Context(), reqLogger)))

			reqLogger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

// chaosMiddleware spares /admin/* so the operator can always switch chaos off
// again, and /healthz so that HTTP latency never gets a pod killed for the wrong
// reason. Liveness has its own switch in a later release.
func chaosMiddleware(store chaos.Store, draw func() float64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/admin/") || r.URL.Path == "/healthz" {
				next.ServeHTTP(w, r)
				return
			}

			s, err := store.Get(r.Context())
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			if d := chaos.Delay(s); d > 0 {
				select {
				case <-time.After(d):
				case <-r.Context().Done():
					return
				}
			}
			if status, fail := chaos.FailWith(s, draw()); fail {
				writeError(w, r, status, "künstlicher Fehler aus der Chaos-Schnittstelle")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func defaultDraw() float64 { return rand.Float64() }
