package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/choffmann/ferry/internal/load"
)

func runLoad(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("load", flag.ContinueOnError)
	fs.SetOutput(stdout)
	target := fs.String("target", "http://localhost:8080", "Adresse der laufenden Instanz")
	rate := fs.Int("rate", 5, "Runden pro Sekunde")
	duration := fs.Duration("duration", time.Minute, "Laufzeit")
	if err := fs.Parse(args); err != nil {
		return err
	}

	res, err := load.Run(ctx, load.Options{
		Target:   *target,
		Rate:     *rate,
		Duration: *duration,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%d Anfragen\n", res.Requests)
	statuses := make([]int, 0, len(res.ByStatus))
	for s := range res.ByStatus {
		statuses = append(statuses, s)
	}
	sort.Ints(statuses)
	for _, s := range statuses {
		fmt.Fprintf(stdout, "  %d: %d\n", s, res.ByStatus[s])
	}
	return nil
}
