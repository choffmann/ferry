package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/choffmann/ferry/internal/config"
	"github.com/choffmann/ferry/internal/obs"
	"github.com/choffmann/ferry/internal/payments"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string, stdout io.Writer) error {
	cfg, err := config.Payments(getenv)
	if err != nil {
		return err
	}

	logger := obs.NewLogger(stdout)
	info := obs.Version()
	if !info.Stamped() {
		logger.Warn("build info missing, /version answers with empty fields, " +
			"stamp the binary with -ldflags at build time")
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return err
	}

	srv := &http.Server{
		Handler:           payments.NewRouter(logger),
		ReadHeaderTimeout: 5 * time.Second,
	}
	logger.Info("payments is starting",
		"addr", ln.Addr().String(), "version", info.Version, "commit", info.Commit)

	serveErr := make(chan error, 1)
	go func() {
		err := srv.Serve(ln)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serveErr <- err
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		logger.Info("payments is stopping")
		return srv.Shutdown(shutdownCtx)
	}
}
