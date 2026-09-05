package domain

import (
	"errors"
	"testing"
)

func TestSeatsAvailable(t *testing.T) {
	d := Departure{Capacity: 40, Booked: 12}
	if got := d.SeatsAvailable(); got != 28 {
		t.Errorf("SeatsAvailable() = %d, want 28", got)
	}
}

func TestCheckSeatsAllowsWhatFits(t *testing.T) {
	d := Departure{Capacity: 40, Booked: 38}
	if err := CheckSeats(d, 2, BookOptions{}); err != nil {
		t.Errorf("CheckSeats for the last two seats: %v", err)
	}
}

func TestCheckSeatsRejectsOneTooMany(t *testing.T) {
	d := Departure{Capacity: 40, Booked: 38}
	if err := CheckSeats(d, 3, BookOptions{}); !errors.Is(err, ErrSoldOut) {
		t.Errorf("CheckSeats = %v, want ErrSoldOut", err)
	}
}

func TestCheckSeatsWithOverbookingIgnoresCapacity(t *testing.T) {
	d := Departure{Capacity: 40, Booked: 40}
	if err := CheckSeats(d, 5, BookOptions{AllowOverbooking: true}); err != nil {
		t.Errorf("CheckSeats with overbooking allowed: %v", err)
	}
}

func TestBookingRequestValidate(t *testing.T) {
	valid := BookingRequest{DepartureID: "FL-SO-2026-09-29T08", Passengers: 2}
	if err := valid.Validate(); err != nil {
		t.Errorf("Validate on a valid request: %v", err)
	}

	invalid := []BookingRequest{
		{DepartureID: "", Passengers: 2},
		{DepartureID: "FL-SO-2026-09-29T08", Passengers: 0},
		{DepartureID: "FL-SO-2026-09-29T08", Passengers: 10},
	}
	for _, r := range invalid {
		if err := r.Validate(); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("Validate(%+v) = %v, want ErrInvalidRequest", r, err)
		}
	}
}
