package httpapi

import (
	"net/http"

	"github.com/choffmann/ferry/internal/httpmw"
)

// The handlers keep the short local spelling; the format itself is shared with
// the payment service and lives in httpmw.
func writeJSON(w http.ResponseWriter, status int, v any) {
	httpmw.WriteJSON(w, status, v)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	httpmw.WriteError(w, r, status, msg)
}
