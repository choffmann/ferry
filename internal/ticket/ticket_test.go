package ticket

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/choffmann/ferry/internal/domain"
)

func sample() Ticket {
	departsAt := time.Date(2026, 10, 9, 6, 0, 0, 0, time.UTC)
	return Ticket{
		Booking: domain.Booking{
			ID:          "bk-000042",
			DepartureID: "FL-SO-2026-10-09T06",
			Passengers:  3,
			Status:      domain.StatusConfirmed,
			CreatedAt:   departsAt.Add(-2 * time.Hour),
		},
		Departure:  domain.Departure{ID: "FL-SO-2026-10-09T06", ConnectionID: "FL-SO", DepartsAt: departsAt},
		Connection: domain.Connections()[0],
		IssuedAt:   departsAt.Add(-time.Hour),
	}
}

func TestLoadReportsAMissingAssetDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "assets")
	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load on a missing asset directory returned no error")
	}
	if !strings.Contains(err.Error(), dir) {
		t.Errorf("error %q does not name the path it looked in", err)
	}
}

func TestRenderFillsInTheBooking(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "tickets"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "{{.Booking.ID}} {{.Booking.Passengers}} {{.Connection.From.Name}}\n"
	if err := os.WriteFile(filepath.Join(dir, "tickets", "ticket.txt.tmpl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var out strings.Builder
	if err := r.Render(&out, sample()); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := out.String(); got != "bk-000042 3 Flensburg\n" {
		t.Errorf("rendered %q", got)
	}
}

func TestBoardingClosesBeforeTheDeparture(t *testing.T) {
	tk := sample()
	want := tk.Departure.DepartsAt.Add(-domain.BoardingClosesBefore)
	if got := tk.BoardingClosesAt(); !got.Equal(want) {
		t.Errorf("BoardingClosesAt() = %s, want %s", got, want)
	}
}

func TestTheShippedTemplateRenders(t *testing.T) {
	r, err := Load(filepath.Join("..", "..", "assets"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var out strings.Builder
	if err := r.Render(&out, sample()); err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{"bk-000042", "Flensburg", "Sønderborg", "confirmed"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the shipped ticket does not mention %q:\n%s", want, out.String())
		}
	}
}
