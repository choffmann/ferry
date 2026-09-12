package main

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestRunWorkerStopsWithoutPaymentsURL(t *testing.T) {
	env := map[string]string{"DATABASE_URL": "postgres://localhost/ferry"}
	err := runWorker(context.Background(), nil, func(k string) string { return env[k] }, io.Discard)
	if err == nil {
		t.Fatal("the worker started without PAYMENTS_URL")
	}
	if !strings.Contains(err.Error(), "PAYMENTS_URL") {
		t.Errorf("the error does not name PAYMENTS_URL: %v", err)
	}
}
