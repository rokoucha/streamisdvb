package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"syscall"

	"github.com/rokoucha/streamisdvb/internal/config"
	"github.com/rokoucha/streamisdvb/internal/dvb"
)

func receiverLogger(log *slog.Logger, c config.Config) *slog.Logger {
	log = log.With("device", c.Device, "delivery_system", c.Delivery, "frequency_hz", c.Frequency,
		"inversion", c.Inversion, "demux", c.Demux, "dvr", c.DVR, "tune_timeout", c.TuneTimeout, "idle_timeout", c.IdleTimeout)
	if c.Channel != "" {
		log = log.With("channel", c.Channel)
	}
	if c.Delivery == "isdb-t" {
		log = log.With("bandwidth_hz", c.Bandwidth)
	} else {
		log = log.With("symbol_rate_ksps", c.SymbolRate/1000)
		if c.Satellite() {
			log = log.With("lnb_voltage", c.LNBVoltage)
			log = log.With("stream_id", fmt.Sprintf("0x%04x", c.StreamID))
		} else {
			log = log.With("modulation", c.Modulation)
		}
	}
	format := "ts"
	if c.Delivery == "isdb-s3" {
		format = "tlv"
	}
	return log.With("format", format)
}

// errors.Is would hide failures joined with cancellation.
func cancellationOnly(err error) bool {
	if err == context.Canceled {
		return true
	}
	switch e := err.(type) {
	case interface{ Unwrap() []error }:
		children := e.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !cancellationOnly(child) {
				return false
			}
		}
		return true
	case interface{ Unwrap() error }:
		return cancellationOnly(e.Unwrap())
	}
	return false
}

func logExit(log *slog.Logger, ctx context.Context, err error) int {
	var sig *shutdownSignal
	if errors.As(context.Cause(ctx), &sig) {
		log = log.With("signal", sig.Error())
	}
	var uncertain *dvb.UnsafeTunerError
	if errors.As(err, &uncertain) {
		log.Error("receiver stopped", "reason", "tuner_state_uncertain", "exit_code", 2, "error", err)
		return 2
	}
	if err == nil || cancellationOnly(err) {
		reason := "completed"
		if err != nil {
			reason = "canceled"
			if sig != nil {
				reason = "signal"
			}
		}
		log.Info("receiver stopped", "reason", reason, "exit_code", 0)
		return 0
	}
	reason := "receive_error"
	switch {
	case errors.Is(err, dvb.ErrTuneTimeout):
		reason = "tune_timeout"
	case errors.Is(err, dvb.ErrStreamTimeout):
		reason = "io_timeout"
	case errors.Is(err, syscall.EPIPE):
		reason = "output_closed"
	case errors.Is(err, io.ErrUnexpectedEOF):
		reason = "input_closed"
	}
	log.Error("receiver stopped", "reason", reason, "exit_code", 1, "error", err)
	return 1
}
