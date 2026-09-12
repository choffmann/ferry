package payments_test

import (
	"testing"

	"github.com/choffmann/ferry/internal/domain"
	"github.com/choffmann/ferry/internal/payments"
)

func TestDecideApprovesUpToTheCardLimit(t *testing.T) {
	res := payments.Decide(payments.ChargeRequest{
		BookingID: "bk-000001", AmountCents: payments.CardLimitCents,
	})
	if res.Status != payments.Approved {
		t.Fatalf("status = %q, want approved", res.Status)
	}
	if res.Reference == "" {
		t.Error("an approval without a reference")
	}
}

func TestDecideDeclinesAboveTheCardLimit(t *testing.T) {
	res := payments.Decide(payments.ChargeRequest{
		BookingID: "bk-000001", AmountCents: payments.CardLimitCents + 1,
	})
	if res.Status != payments.Declined {
		t.Fatalf("status = %q, want declined", res.Status)
	}
	if res.Reason == "" {
		t.Error("a decline without a reason")
	}
	if res.Reference != "" {
		t.Error("a decline with a reference")
	}
}

// The limit is the reason large groups fail, so it has to sit between the fare
// of six and of seven passengers.
func TestTheCardLimitBitesFromSevenPassengers(t *testing.T) {
	if domain.FareCents(6) > payments.CardLimitCents {
		t.Errorf("six passengers already exceed the limit")
	}
	if domain.FareCents(7) <= payments.CardLimitCents {
		t.Errorf("seven passengers stay below the limit")
	}
}
