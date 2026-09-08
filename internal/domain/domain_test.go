package domain

import (
	"errors"
	"testing"
	"time"
)

var testNow = time.Date(2026, 9, 29, 6, 0, 0, 0, time.UTC)

func departureIn(d time.Duration, capacity, booked int) Departure {
	return Departure{DepartsAt: testNow.Add(d), Capacity: capacity, Booked: booked}
}

func TestSeatsAvailable(t *testing.T) {
	d := Departure{Capacity: 40, Booked: 12}
	if got := d.SeatsAvailable(); got != 28 {
		t.Errorf("SeatsAvailable() = %d, want 28", got)
	}
}

func TestCheckBookableAllowsWhatFits(t *testing.T) {
	d := departureIn(2*time.Hour, 40, 38)
	if err := CheckBookable(d, testNow, 2, BookOptions{}); err != nil {
		t.Errorf("CheckBookable for the last two seats: %v", err)
	}
}

func TestCheckBookableRejectsOneTooMany(t *testing.T) {
	d := departureIn(2*time.Hour, 40, 38)
	if err := CheckBookable(d, testNow, 3, BookOptions{}); !errors.Is(err, ErrSoldOut) {
		t.Errorf("CheckBookable = %v, want ErrSoldOut", err)
	}
}

func TestCheckBookableWithOverbookingIgnoresCapacity(t *testing.T) {
	d := departureIn(2*time.Hour, 40, 40)
	if err := CheckBookable(d, testNow, 5, BookOptions{AllowOverbooking: true}); err != nil {
		t.Errorf("CheckBookable with overbooking allowed: %v", err)
	}
}

func TestCheckBookableRejectsADepartureThatHasLeft(t *testing.T) {
	d := departureIn(-time.Hour, 40, 0)
	if err := CheckBookable(d, testNow, 1, BookOptions{}); !errors.Is(err, ErrBookingClosed) {
		t.Errorf("CheckBookable on a departure that left an hour ago = %v, want ErrBookingClosed", err)
	}
}

func TestCheckBookableRejectsInsideTheBoardingWindow(t *testing.T) {
	d := departureIn(BoardingClosesBefore-time.Minute, 40, 0)
	if err := CheckBookable(d, testNow, 1, BookOptions{}); !errors.Is(err, ErrBookingClosed) {
		t.Errorf("CheckBookable inside the boarding window = %v, want ErrBookingClosed", err)
	}
}

func TestOverbookingDoesNotReopenAClosedDeparture(t *testing.T) {
	d := departureIn(-time.Hour, 40, 0)
	if err := CheckBookable(d, testNow, 1, BookOptions{AllowOverbooking: true}); !errors.Is(err, ErrBookingClosed) {
		t.Errorf("CheckBookable = %v, want ErrBookingClosed even with overbooking allowed", err)
	}
}

func TestBookingOpenEndsWhenBoardingCloses(t *testing.T) {
	d := departureIn(BoardingClosesBefore, 40, 0)
	if !BookingOpen(d, testNow.Add(-time.Second)) {
		t.Error("BookingOpen is false a second before boarding closes")
	}
	if BookingOpen(d, testNow) {
		t.Error("BookingOpen is true at the moment boarding closes")
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
