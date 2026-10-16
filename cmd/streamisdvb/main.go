package main

import (
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/rokoucha/streamisdvb/internal/config"
	"github.com/rokoucha/streamisdvb/internal/dvb"
)

func main() {
	// A broken log pipe must not bypass DVB cleanup.
	signal.Ignore(syscall.SIGPIPE)
	os.Exit(run(os.Args[1:], os.Stderr))
}
func run(args []string, stderr io.Writer) int {
	log := slog.New(slog.NewTextHandler(stderr, nil))
	c, err := config.Parse(args, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		log.Error("invalid arguments", "device", c.Device, "reason", "invalid_arguments", "exit_code", 1, "error", err)
		return 1
	}
	ctx, stop := signalContext()
	defer stop()
	log = receiverLogger(log, c)
	log.Info("receiver starting")
	err = dvb.Receive(ctx, c, log)
	return logExit(log, ctx, err)
}
