package httpapi

import (
	"encoding/json"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorBody struct {
	Error     string `json:"error"`
	RequestID string `json:"request_id"`
}

// The recoverer sits outside the request-id middleware, so on a recovered panic
// its request predates the id. The response header is shared across the chain
// and still carries it.
func requestIDFor(w http.ResponseWriter, r *http.Request) string {
	if id := requestIDFrom(r.Context()); id != "" {
		return id
	}
	return w.Header().Get("X-Request-Id")
}

func writeError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	writeJSON(w, status, errorBody{Error: msg, RequestID: requestIDFor(w, r)})
}
