// Package ticket renders boarding passes from the templates in the asset
// directory. The templates are read at startup, not compiled into the binary:
// the deployable artefact is the binary plus its assets.
package ticket

import (
	"fmt"
	"io"
	"path/filepath"
	"text/template"
	"time"

	"github.com/choffmann/ferry/internal/domain"
)

const templateFile = "tickets/ticket.txt.tmpl"

type Renderer struct {
	tmpl *template.Template
}

func Load(assetsDir string) (*Renderer, error) {
	path := filepath.Join(assetsDir, templateFile)
	tmpl, err := template.ParseFiles(path)
	if err != nil {
		return nil, fmt.Errorf("ticket template %s: %w", path, err)
	}
	return &Renderer{tmpl: tmpl}, nil
}

type Ticket struct {
	Booking    domain.Booking
	Departure  domain.Departure
	Connection domain.Connection
	IssuedAt   time.Time
}

func (t Ticket) BoardingClosesAt() time.Time {
	return t.Departure.DepartsAt.Add(-domain.BoardingClosesBefore)
}

func (r *Renderer) Render(w io.Writer, t Ticket) error {
	return r.tmpl.Execute(w, t)
}
