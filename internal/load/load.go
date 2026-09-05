package load

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/choffmann/ferry/internal/domain"
)

type Options struct {
	Target   string
	Rate     int
	Duration time.Duration
	Client   *http.Client
}

type Result struct {
	Requests int
	ByStatus map[int]int
}

func Run(ctx context.Context, o Options) (Result, error) {
	if o.Target == "" {
		return Result{}, errors.New("kein Ziel angegeben")
	}
	if o.Rate < 1 {
		o.Rate = 1
	}
	if o.Client == nil {
		o.Client = &http.Client{Timeout: 10 * time.Second}
	}

	ctx, cancel := context.WithTimeout(ctx, o.Duration)
	defer cancel()

	conns, err := fetchConnections(ctx, o)
	if err != nil {
		return Result{}, err
	}
	if len(conns) == 0 {
		return Result{}, errors.New("das Ziel meldet keine Verbindungen")
	}

	res := Result{ByStatus: map[int]int{}}
	var mu sync.Mutex
	record := func(status int) {
		mu.Lock()
		defer mu.Unlock()
		res.Requests++
		res.ByStatus[status]++
	}

	ticker := time.NewTicker(time.Second / time.Duration(o.Rate))
	defer ticker.Stop()

	var wg sync.WaitGroup
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return res, nil
		case <-ticker.C:
			wg.Add(1)
			go func() {
				defer wg.Done()
				c := conns[rand.IntN(len(conns))]
				oneRound(ctx, o, c, record)
			}()
		}
	}
}

func oneRound(ctx context.Context, o Options, c domain.Connection, record func(int)) {
	departures, status, err := fetchDepartures(ctx, o, c.ID)
	if err != nil {
		return
	}
	record(status)
	if len(departures) == 0 {
		return
	}

	d := departures[rand.IntN(len(departures))]
	body, err := json.Marshal(domain.BookingRequest{
		DepartureID: d.ID,
		Passengers:  1 + rand.IntN(3),
	})
	if err != nil {
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.Target+"/bookings", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.Client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var b domain.Booking
	if resp.StatusCode == http.StatusCreated {
		err = json.NewDecoder(resp.Body).Decode(&b)
	} else {
		io.Copy(io.Discard, resp.Body)
	}
	record(resp.StatusCode)
	if err != nil || resp.StatusCode != http.StatusCreated || rand.IntN(4) != 0 {
		return
	}

	readReq, err := http.NewRequestWithContext(ctx, http.MethodGet, o.Target+"/bookings/"+b.ID, nil)
	if err != nil {
		return
	}
	readResp, err := o.Client.Do(readReq)
	if err != nil {
		return
	}
	defer readResp.Body.Close()
	io.Copy(io.Discard, readResp.Body)
	record(readResp.StatusCode)
}

func fetchConnections(ctx context.Context, o Options) ([]domain.Connection, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.Target+"/connections", nil)
	if err != nil {
		return nil, err
	}
	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out []domain.Connection
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("Antwort von %s/connections nicht lesbar: %w", o.Target, err)
	}
	return out, nil
}

func fetchDepartures(ctx context.Context, o Options, connectionID string) ([]domain.Departure, int, error) {
	url := o.Target + "/connections/" + connectionID + "/departures"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	var out []domain.Departure
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, resp.StatusCode, nil
	}
	return out, resp.StatusCode, nil
}
