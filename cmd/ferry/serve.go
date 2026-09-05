package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/choffmann/ferry/internal/chaos"
	"github.com/choffmann/ferry/internal/config"
	"github.com/choffmann/ferry/internal/httpapi"
	"github.com/choffmann/ferry/internal/obs"
	"github.com/choffmann/ferry/internal/store"
)

const leakInterval = 6 * time.Second

func runServe(ctx context.Context, args []string, getenv func(string) string, stdout io.Writer) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stdout)
	addr := fs.String("addr", "", "Adresse zum Lauschen, überschreibt PORT")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}

	logger := obs.NewLogger(stdout)
	if cfg.AdminTokenIsDefault {
		logger.Warn("ADMIN_TOKEN nicht gesetzt, es gilt der Vorgabewert",
			"admin_token", config.DefaultAdminToken)
	}

	chaosStore := chaos.NewMemoryStore()
	leaker := chaos.NewLeaker(chaosStore, leakInterval)
	go leaker.Run(ctx)

	handler := httpapi.NewRouter(httpapi.Deps{
		Repo:       store.NewMemoryStore(time.Now().UTC()),
		Chaos:      chaosStore,
		AdminToken: cfg.AdminToken,
		Logger:     logger,
	})

	listenOn := *addr
	if listenOn == "" {
		listenOn = fmt.Sprintf(":%d", cfg.Port)
	}
	ln, err := net.Listen("tcp", listenOn)
	if err != nil {
		return err
	}

	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	info := obs.Version()
	logger.Info("ferry startet",
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
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		logger.Info("ferry hält an")
		return srv.Shutdown(shutdownCtx)
	}
}
