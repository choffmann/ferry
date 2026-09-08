package domain

import (
	"fmt"
	"time"
)

const (
	SeatsPerDeparture    = 40
	BoardingClosesBefore = 15 * time.Minute

	seedDays  = 7
	firstHour = 6
	lastHour  = 20
	hourStep  = 2
)

func Ports() []Port {
	return []Port{
		{Code: "FL", Name: "Flensburg"},
		{Code: "SO", Name: "Sønderborg"},
		{Code: "GL", Name: "Glücksburg"},
	}
}

func port(code string) Port {
	for _, p := range Ports() {
		if p.Code == code {
			return p
		}
	}
	panic("unknown port " + code)
}

func Connections() []Connection {
	return []Connection{
		{ID: "FL-SO", From: port("FL"), To: port("SO"), DurationMinutes: 95},
		{ID: "SO-FL", From: port("SO"), To: port("FL"), DurationMinutes: 95},
		{ID: "FL-GL", From: port("FL"), To: port("GL"), DurationMinutes: 35},
		{ID: "GL-FL", From: port("GL"), To: port("FL"), DurationMinutes: 35},
	}
}

// Departures builds the timetable for the seven days that start with now's date.
// Deriving it from the date rather than from a fixed epoch keeps the data
// identical across teams without ever going stale.
func Departures(now time.Time) []Departure {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	var out []Departure
	for _, c := range Connections() {
		for d := 0; d < seedDays; d++ {
			for h := firstHour; h <= lastHour; h += hourStep {
				at := day.AddDate(0, 0, d).Add(time.Duration(h) * time.Hour)
				out = append(out, Departure{
					ID:           fmt.Sprintf("%s-%s", c.ID, at.Format("2006-01-02T15")),
					ConnectionID: c.ID,
					DepartsAt:    at,
					Capacity:     SeatsPerDeparture,
				})
			}
		}
	}
	return out
}
