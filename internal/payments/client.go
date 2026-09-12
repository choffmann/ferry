package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		http:    &http.Client{Timeout: timeout},
	}
}

// Charge returns an error for everything that is not an answer about the
// payment. A decline is not an error, it comes back in the response.
func (c *Client) Charge(ctx context.Context, cr ChargeRequest) (ChargeResponse, error) {
	body, err := json.Marshal(cr)
	if err != nil {
		return ChargeResponse{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/charge", bytes.NewReader(body))
	if err != nil {
		return ChargeResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return ChargeResponse{}, fmt.Errorf("asking the payment service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ChargeResponse{}, fmt.Errorf("payment service answered %d", resp.StatusCode)
	}

	var out ChargeResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ChargeResponse{}, fmt.Errorf("reading the answer of the payment service: %w", err)
	}
	if out.Status != Approved && out.Status != Declined {
		return ChargeResponse{}, fmt.Errorf("payment service answered with status %q", out.Status)
	}
	return out, nil
}
