package main

import (
	"context"
	"errors"
	"os"
	"os/signal"

	"github.com/kayac/gcrunpresso/v2"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), trapSignals...)
	defer stop()

	exitCode, err := gcrunpresso.CLI(ctx, gcrunpresso.ParseCLIv2)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			gcrunpresso.LogWarn("Interrupted")
		} else {
			gcrunpresso.LogError("FAILED. %s", err)
		}
	}
	os.Exit(exitCode)
}
