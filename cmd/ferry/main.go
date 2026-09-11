package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "serve":
		err = runServe(ctx, os.Args[2:], os.Getenv, os.Stdout)
	case "migrate":
		err = runMigrate(ctx, os.Args[2:], os.Getenv, os.Stdout)
	case "seed":
		err = runSeed(ctx, os.Args[2:], os.Getenv, os.Stdout)
	case "load":
		err = runLoad(ctx, os.Args[2:], os.Stdout)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `ferry <command>

  serve     starts the API server
  migrate   applies the pending database migrations
  seed      writes the connections and the timetable
  load      generates load against a running instance
`)
}
