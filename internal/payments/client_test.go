package payments_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/choffmann/ferry/internal/payments"
)

func TestDeclineIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"declined","reason":"amount exceeds the card limit"}`))
	}))
	defer srv.Close()

	client := payments.NewClient(srv.URL, time.Second)
	res, err := client.Charge(context.Background(), payments.ChargeRequest{
		BookingID: "bk-test", AmountCents: 99999,
	})

	if err != nil {
		t.Fatalf("error should be nil, got %v", err)
	}
	if res.Status != payments.Declined {
		t.Errorf("status = %q, want declined", res.Status)
	}
	if res.Reason != "amount exceeds the card limit" {
		t.Errorf("reason = %q, want the decline reason", res.Reason)
	}
}

func TestApprovalRoundTrips(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/charge" {
			t.Errorf("path = %s, want /charge", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		if got := string(body); got != `{"booking_id":"bk-test","amount_cents":5000}` {
			t.Errorf("request body = %s, want json with booking id and amount", got)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"approved","reference":"pay-abc123"}`))
	}))
	defer srv.Close()

	client := payments.NewClient(srv.URL, time.Second)
	res, err := client.Charge(context.Background(), payments.ChargeRequest{
		BookingID: "bk-test", AmountCents: 5000,
	})

	if err != nil {
		t.Fatalf("error should be nil, got %v", err)
	}
	if res.Status != payments.Approved {
		t.Errorf("status = %q, want approved", res.Status)
	}
	if res.Reference != "pay-abc123" {
		t.Errorf("reference = %q, want pay-abc123", res.Reference)
	}
}

func TestNonTwoHundredAnswerIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := payments.NewClient(srv.URL, time.Second)
	res, err := client.Charge(context.Background(), payments.ChargeRequest{
		BookingID: "bk-test", AmountCents: 5000,
	})

	if err == nil {
		t.Fatalf("error should not be nil")
	}
	if res != (payments.ChargeResponse{}) {
		t.Errorf("response should be zero value, got %+v", res)
	}
	if got := err.Error(); len(got) == 0 || got[len(got)-3:] != "502" {
		t.Errorf("error message should mention 502, got %q", err.Error())
	}
}

func TestUnreadableBodyIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{`))
	}))
	defer srv.Close()

	client := payments.NewClient(srv.URL, time.Second)
	res, err := client.Charge(context.Background(), payments.ChargeRequest{
		BookingID: "bk-test", AmountCents: 5000,
	})

	if err == nil {
		t.Fatalf("error should not be nil")
	}
	if res != (payments.ChargeResponse{}) {
		t.Errorf("response should be zero value, got %+v", res)
	}
}

func TestUnknownStatusIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"maybe"}`))
	}))
	defer srv.Close()

	client := payments.NewClient(srv.URL, time.Second)
	res, err := client.Charge(context.Background(), payments.ChargeRequest{
		BookingID: "bk-test", AmountCents: 5000,
	})

	if err == nil {
		t.Fatalf("error should not be nil")
	}
	if res != (payments.ChargeResponse{}) {
		t.Errorf("response should be zero value, got %+v", res)
	}
}
