// Package payments is the second service of the system. It answers whether a
// booking may be charged. It holds no state and knows no database.
package payments

import "github.com/google/uuid"

// CardLimitCents is a business rule, not a property of the environment the
// service runs in, so it is a constant rather than configuration.
const CardLimitCents = 7500

type Status string

const (
	Approved Status = "approved"
	Declined Status = "declined"
)

type ChargeRequest struct {
	BookingID   string `json:"booking_id"`
	AmountCents int    `json:"amount_cents"`
}

type ChargeResponse struct {
	Status    Status `json:"status"`
	Reference string `json:"reference,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

func Decide(req ChargeRequest) ChargeResponse {
	if req.AmountCents > CardLimitCents {
		return ChargeResponse{Status: Declined, Reason: "amount exceeds the card limit"}
	}
	return ChargeResponse{Status: Approved, Reference: "pay-" + uuid.NewString()}
}
