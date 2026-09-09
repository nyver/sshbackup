package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// runConsole runs the service in the foreground, with the same startup
// and shutdown sequence as service mode. Ctrl+C (or a terminate signal)
// triggers the same graceful shutdown as a Windows Service stop request.
func runConsole() error {
	ctx := context.Background()
	app, err := startup(ctx, "console")
	if err != nil {
		return err
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	app.Logger.Info("running in console mode; press Ctrl+C to stop")
	<-sigCh

	app.Shutdown()
	return nil
}
