package httpapi

import (
	"bytes"
	"context"
	_ "embed"
	"html/template"
	"net/http"
	"sync"

	"github.com/choffmann/ferry/internal/obs"
)

//go:embed index.html
var indexSource string

var indexPage = template.Must(template.New("index").Parse(indexSource))

const (
	statusReachable     = "reachable"
	statusUnreachable   = "unreachable"
	statusNotConfigured = "not configured"
)

type serviceStatus struct {
	Name   string
	Status string
}

type indexData struct {
	Build    obs.BuildInfo
	Services []serviceStatus
}

func (d Deps) index(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
	defer cancel()

	probes := []struct {
		name string
		ping func(context.Context) error
	}{
		{"database", d.Ping},
		{"payments", d.PaymentsPing},
	}

	services := make([]serviceStatus, len(probes))
	var wg sync.WaitGroup
	for i, p := range probes {
		services[i] = serviceStatus{Name: p.name, Status: statusNotConfigured}
		if p.ping == nil {
			continue
		}
		wg.Go(func() {
			if err := p.ping(ctx); err != nil {
				obs.LoggerFrom(r.Context()).Warn("service is not reachable",
					"service", p.name, "error", err)
				services[i].Status = statusUnreachable
				return
			}
			services[i].Status = statusReachable
		})
	}
	wg.Wait()

	var buf bytes.Buffer
	if err := indexPage.Execute(&buf, indexData{Build: obs.Version(), Services: services}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "start page could not be rendered")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}
