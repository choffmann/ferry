package load

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/choffmann/ferry/internal/domain"
)

func fakeAPI(t *testing.T, bookings, bookingReads *atomic.Int64) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /connections", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(domain.Connections())
	})
	mux.HandleFunc("GET /connections/{id}/departures", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]domain.Departure{{
			ID:           r.PathValue("id") + "-2026-09-29T06",
			ConnectionID: r.PathValue("id"),
			DepartsAt:    time.Date(2026, 9, 29, 6, 0, 0, 0, time.UTC),
			Capacity:     40,
		}})
	})
	mux.HandleFunc("POST /bookings", func(w http.ResponseWriter, r *http.Request) {
		bookings.Add(1)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(domain.Booking{ID: "bk-000001", Status: domain.StatusConfirmed})
	})
	mux.HandleFunc("GET /bookings/{id}", func(w http.ResponseWriter, r *http.Request) {
		bookingReads.Add(1)
		json.NewEncoder(w).Encode(domain.Booking{ID: r.PathValue("id"), Status: domain.StatusConfirmed})
	})
	return httptest.NewServer(mux)
}

func TestRunBooksAndCountsByStatus(t *testing.T) {
	var bookings, bookingReads atomic.Int64
	srv := fakeAPI(t, &bookings, &bookingReads)
	defer srv.Close()

	res, err := Run(context.Background(), Options{
		Target:   srv.URL,
		Rate:     50,
		Duration: 200 * time.Millisecond,
		Client:   srv.Client(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Requests == 0 {
		t.Fatal("no requests were made")
	}
	if bookings.Load() == 0 {
		t.Error("no booking was attempted")
	}
	if res.ByStatus[http.StatusCreated] == 0 {
		t.Errorf("no 201 counted: %+v", res.ByStatus)
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	var bookings, bookingReads atomic.Int64
	srv := fakeAPI(t, &bookings, &bookingReads)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	if _, err := Run(ctx, Options{
		Target:   srv.URL,
		Rate:     10,
		Duration: time.Hour,
		Client:   srv.Client(),
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("Run ran for %v after the cancel", elapsed)
	}
}

func TestRunRejectsAnEmptyTarget(t *testing.T) {
	if _, err := Run(context.Background(), Options{Rate: 1, Duration: time.Second}); err == nil {
		t.Error("Run without a target returned no error")
	}
}

func TestRunReadsBookingsBack(t *testing.T) {
	var bookings, bookingReads atomic.Int64
	srv := fakeAPI(t, &bookings, &bookingReads)
	defer srv.Close()

	_, err := Run(context.Background(), Options{
		Target:   srv.URL,
		Rate:     200,
		Duration: time.Second,
		Client:   srv.Client(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if bookings.Load() == 0 {
		t.Fatal("no booking was attempted")
	}
	if bookingReads.Load() == 0 {
		t.Errorf("no booking was ever read back out of %d bookings", bookings.Load())
	}
}
