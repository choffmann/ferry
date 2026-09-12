package httpmw

import (
	"encoding/json"
	"net/http"
)

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ErrorBody is the error format docs/api.md describes. It is exported because
// the tests of both services decode into it.
type ErrorBody struct {
	Error     string `json:"error"`
	RequestID string `json:"request_id"`
}

// The recoverer sits outside the request-id middleware, so on a recovered panic
// its request predates the id. The response header is shared across the chain
// and still carries it.
func RequestIDFor(w http.ResponseWriter, r *http.Request) string {
	if id := RequestIDFrom(r.Context()); id != "" {
		return id
	}
	return w.Header().Get("X-Request-Id")
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	WriteJSON(w, status, ErrorBody{Error: msg, RequestID: RequestIDFor(w, r)})
}
