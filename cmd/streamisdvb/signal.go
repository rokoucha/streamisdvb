package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func signalContext() (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(context.Background())
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case sig := <-signals:
			cancel(&shutdownSignal{sig})
		case <-ctx.Done():
		}
	}()
	return ctx, func() { signal.Stop(signals); cancel(nil) }
}

type shutdownSignal struct{ signal os.Signal }

func (e *shutdownSignal) Error() string {
	switch e.signal {
	case os.Interrupt:
		return "SIGINT"
	case syscall.SIGTERM:
		return "SIGTERM"
	}
	return e.signal.String()
}
