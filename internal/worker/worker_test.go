package worker_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/choffmann/ferry/internal/domain"
	"github.com/choffmann/ferry/internal/obs"
	"github.com/choffmann/ferry/internal/payments"
	"github.com/choffmann/ferry/internal/store"
	"github.com/choffmann/ferry/internal/worker"
)

// The fake checks the boarding deadline against the real clock, so the
// timetable has to be seeded from today rather than from a fixed date.
func newFake() *store.Fake { return store.NewFake(time.Now().UTC()) }

func bookedFake(t *testing.T, passengers int) (*store.Fake, domain.Booking) {
	t.Helper()
	f := newFake()
	dep, err := f.Departures(context.Background(), "FL-SO", time.Time{})
	if err != nil || len(dep) == 0 {
		t.Fatalf("Departures: %v (%d)", err, len(dep))
	}
	b, err := f.Book(context.Background(),
		domain.BookingRequest{DepartureID: dep[0].ID, Passengers: passengers},
		domain.BookOptions{})
	if err != nil {
		t.Fatalf("Book: %v", err)
	}
	return f, b
}

func newWorker(q store.Queue, url string) *worker.Worker {
	return worker.New(q, payments.NewClient(url, 2*time.Second),
		obs.NewLogger(&bytes.Buffer{}))
}

func TestTickConfirmsASmallBooking(t *testing.T) {
	srv := httptest.NewServer(payments.NewRouter(obs.NewLogger(&bytes.Buffer{})))
	defer srv.Close()

	f, b := bookedFake(t, 2)
	n, err := newWorker(f, srv.URL).Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if n != 1 {
		t.Fatalf("settled %d bookings, want 1", n)
	}

	got, err := f.Booking(context.Background(), b.ID)
	if err != nil {
		t.Fatalf("Booking: %v", err)
	}
	if got.Status != domain.StatusConfirmed {
		t.Errorf("status = %q, want confirmed", got.Status)
	}
	if got.PaymentRef == "" {
		t.Error("a confirmed booking without a payment reference")
	}
}

func TestTickFailsABookingOverTheCardLimitAndFreesTheSeats(t *testing.T) {
	srv := httptest.NewServer(payments.NewRouter(obs.NewLogger(&bytes.Buffer{})))
	defer srv.Close()

	f, b := bookedFake(t, 8)
	before, err := f.Departure(context.Background(), b.DepartureID)
	if err != nil {
		t.Fatalf("Departure: %v", err)
	}

	if _, err := newWorker(f, srv.URL).Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	got, err := f.Booking(context.Background(), b.ID)
	if err != nil {
		t.Fatalf("Booking: %v", err)
	}
	if got.Status != domain.StatusFailed {
		t.Fatalf("status = %q, want failed", got.Status)
	}
	if got.FailureReason == "" {
		t.Error("a failed booking without a reason")
	}

	after, err := f.Departure(context.Background(), b.DepartureID)
	if err != nil {
		t.Fatalf("Departure: %v", err)
	}
	if after.Booked != before.Booked-8 {
		t.Errorf("booked = %d, want %d", after.Booked, before.Booked-8)
	}
}

// An outage of the payment service is not a statement about the payment. The
// booking has to stay open and has to be claimable again.
func TestTickLeavesTheBookingOpenWhenThePaymentServiceIsDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	f, b := bookedFake(t, 2)
	w := newWorker(f, srv.URL)
	if _, err := w.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	got, err := f.Booking(context.Background(), b.ID)
	if err != nil {
		t.Fatalf("Booking: %v", err)
	}
	if got.Status != domain.StatusPending {
		t.Fatalf("status = %q, want pending", got.Status)
	}

	claimed, err := f.Claim(context.Background(), 10)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if len(claimed) != 1 {
		t.Errorf("the booking could not be claimed again: %+v", claimed)
	}
}

// stubCharger is a hand-written Charger: the real payments.Client can never
// return a status other than approved or declined, but any other
// implementation of the exported Charger interface can.
type stubCharger struct {
	res payments.ChargeResponse
}

func (s stubCharger) Charge(context.Context, payments.ChargeRequest) (payments.ChargeResponse, error) {
	return s.res, nil
}

func TestTickLeavesTheBookingOpenOnAnUnrecognisedStatus(t *testing.T) {
	f, b := bookedFake(t, 2)
	w := worker.New(f, stubCharger{res: payments.ChargeResponse{Status: "pending"}},
		obs.NewLogger(&bytes.Buffer{}))

	if _, err := w.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	got, err := f.Booking(context.Background(), b.ID)
	if err != nil {
		t.Fatalf("Booking: %v", err)
	}
	if got.Status != domain.StatusPending {
		t.Fatalf("status = %q, want pending", got.Status)
	}

	claimed, err := f.Claim(context.Background(), 10)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if len(claimed) != 1 {
		t.Errorf("the booking could not be claimed again: %+v", claimed)
	}
}

func TestTickReportsNothingToDoOnAnEmptyQueue(t *testing.T) {
	srv := httptest.NewServer(payments.NewRouter(obs.NewLogger(&bytes.Buffer{})))
	defer srv.Close()

	n, err := newWorker(newFake(), srv.URL).Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if n != 0 {
		t.Errorf("settled %d bookings, want 0", n)
	}
}

func TestRunStopsWithItsContext(t *testing.T) {
	srv := httptest.NewServer(payments.NewRouter(obs.NewLogger(&bytes.Buffer{})))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := newWorker(newFake(), srv.URL).Run(ctx); err != nil {
		t.Errorf("Run: %v", err)
	}
}
