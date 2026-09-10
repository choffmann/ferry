package domain

import (
	"testing"
	"time"
)

func TestPortsAndConnections(t *testing.T) {
	if got := len(Ports()); got != 3 {
		t.Errorf("len(Ports()) = %d, want 3", got)
	}
	if got := len(Connections()); got != 4 {
		t.Errorf("len(Connections()) = %d, want 4", got)
	}
	for _, c := range Connections() {
		if c.From.Code == c.To.Code {
			t.Errorf("connection %s goes nowhere", c.ID)
		}
		if c.DurationMinutes <= 0 {
			t.Errorf("connection %s has no duration", c.ID)
		}
	}
}

func TestDeparturesCount(t *testing.T) {
	now := time.Date(2026, 9, 29, 11, 17, 0, 0, time.UTC)
	got := Departures(now)
	// 4 connections * 7 days * 8 slots between 06:00 and 20:00
	if len(got) != 224 {
		t.Fatalf("len(Departures()) = %d, want 224", len(got))
	}
	for _, d := range got {
		if d.Capacity != SeatsPerDeparture {
			t.Fatalf("departure %s has capacity %d, want %d", d.ID, d.Capacity, SeatsPerDeparture)
		}
		if d.Booked != 0 {
			t.Fatalf("departure %s starts with %d booked", d.ID, d.Booked)
		}
	}
}

func TestDeparturesAreDeterministic(t *testing.T) {
	now := time.Date(2026, 9, 29, 11, 17, 0, 0, time.UTC)
	later := time.Date(2026, 9, 29, 23, 59, 0, 0, time.UTC)
	a, b := Departures(now), Departures(later)
	if len(a) != len(b) {
		t.Fatalf("different lengths: %d and %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("departure %d differs within the same day: %+v and %+v", i, a[i], b[i])
		}
	}
}

func TestDepartureIDsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range Departures(time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)) {
		if seen[d.ID] {
			t.Fatalf("duplicate departure id %s", d.ID)
		}
		seen[d.ID] = true
	}
}

func mustZone(t *testing.T) *time.Location {
	t.Helper()
	loc, err := LoadZone()
	if err != nil {
		t.Fatalf("LoadZone: %v", err)
	}
	return loc
}

func TestLoadZoneReturnsTheTimetableZone(t *testing.T) {
	if got := mustZone(t).String(); got != TimetableZone {
		t.Errorf("LoadZone() = %s, want %s", got, TimetableZone)
	}
}

func TestDeparturesFollowTheZoneOfTheSeed(t *testing.T) {
	berlin := mustZone(t)
	got := Departures(time.Date(2026, 7, 1, 11, 17, 0, 0, berlin))

	first := got[0]
	if first.DepartsAt.Hour() != firstHour {
		t.Errorf("first slot leaves at %02d local, want %02d", first.DepartsAt.Hour(), firstHour)
	}
	if _, offset := first.DepartsAt.Zone(); offset != 2*60*60 {
		t.Errorf("offset in July = %d seconds, want 7200 for summer time in %s", offset, TimetableZone)
	}
}

func TestDeparturesKeepTheirWallClockAcrossTheDstChange(t *testing.T) {
	// The seeded week starting on 22.10.2026 contains the change back to
	// standard time on the 25th.
	for _, d := range Departures(time.Date(2026, 10, 22, 8, 0, 0, 0, mustZone(t))) {
		h := d.DepartsAt.Hour()
		if h < firstHour || h > lastHour || (h-firstHour)%hourStep != 0 {
			t.Fatalf("departure %s leaves at %02d local, off the timetable grid", d.ID, h)
		}
	}
}
