package httpapi

import (
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/choffmann/ferry/internal/chaos"
)

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
				writeError(w, r, status, "artificial error from the chaos interface")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func defaultDraw() float64 { return rand.Float64() }
