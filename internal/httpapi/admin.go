package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/choffmann/ferry/internal/chaos"
)

func (d Deps) authorized(w http.ResponseWriter, r *http.Request) bool {
	header := r.Header.Get("Authorization")
	token := strings.TrimPrefix(header, "Bearer ")
	if token == header || subtle.ConstantTimeCompare([]byte(token), []byte(d.AdminToken)) != 1 {
		writeError(w, r, http.StatusUnauthorized, "gültiges Bearer-Token erforderlich")
		return false
	}
	return true
}

func (d Deps) getChaos(w http.ResponseWriter, r *http.Request) {
	if !d.authorized(w, r) {
		return
	}
	s, err := d.Chaos.Get(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "Chaos-Zustand nicht lesbar")
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (d Deps) postChaos(w http.ResponseWriter, r *http.Request) {
	if !d.authorized(w, r) {
		return
	}
	var p chaos.Patch
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, r, http.StatusBadRequest, "Körper ist kein gültiges JSON")
		return
	}
	s, err := d.Chaos.Apply(r.Context(), p)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "Chaos-Zustand nicht setzbar")
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (d Deps) deleteChaos(w http.ResponseWriter, r *http.Request) {
	if !d.authorized(w, r) {
		return
	}
	s, err := d.Chaos.Reset(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "Chaos-Zustand nicht zurücksetzbar")
		return
	}
	writeJSON(w, http.StatusOK, s)
}
