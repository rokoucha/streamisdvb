package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rokoucha/streamisdvb/internal/config"
	"github.com/rokoucha/streamisdvb/internal/dvb"
)

func TestLogExit(t *testing.T) {
	for _, tc := range []struct {
		name          string
		err           error
		code          int
		level, reason string
	}{
		{"completed", nil, 0, "INFO", "completed"},
		{"signal", errors.Join(fmt.Errorf("read: %w", context.Canceled)), 0, "INFO", "signal"},
		{"lock timeout", dvb.ErrTuneTimeout, 1, "ERROR", "tune_timeout"},
		{"I/O timeout", dvb.ErrStreamTimeout, 1, "ERROR", "io_timeout"},
		{"broken pipe", fmt.Errorf("write: %w", syscall.EPIPE), 1, "ERROR", "output_closed"},
		{"EOF", io.ErrUnexpectedEOF, 1, "ERROR", "input_closed"},
		{"cancel and error", errors.Join(context.Canceled, syscall.EIO), 1, "ERROR", "receive_error"},
		{"cancel and cleanup failure", errors.Join(context.Canceled, &dvb.UnsafeTunerError{Err: syscall.EIO}), 2, "ERROR", "tuner_state_uncertain"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			cancel(&shutdownSignal{syscall.SIGTERM})
			code := logExit(slog.New(slog.NewJSONHandler(&buf, nil)).With("device", "/dev/dvb/adapter2/frontend0"), ctx, tc.err)
			var record map[string]any
			if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
				t.Fatal(err)
			}
			if code != tc.code || record["level"] != tc.level || record["reason"] != tc.reason || record["device"] != "/dev/dvb/adapter2/frontend0" || record["signal"] != "SIGTERM" {
				t.Fatalf("code=%d log=%s", code, buf.String())
			}
			if _, err := time.Parse(time.RFC3339Nano, record["time"].(string)); err != nil {
				t.Fatal(err)
			}
			if tc.code == 0 && record["error"] != nil {
				t.Fatalf("normal shutdown logged error: %s", buf.String())
			}
		})
	}
}
func TestReceiverLogConditions(t *testing.T) {
	for _, channel := range []string{"27", "C53", "0x40f1", "0xb110"} {
		c, err := config.Parse([]string{channel}, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		receiverLogger(slog.New(slog.NewJSONHandler(&buf, nil)), c).Info("receiver starting")
		var record map[string]any
		if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		if record["channel"] != channel || record["frequency_hz"] != float64(c.Frequency) || record["delivery_system"] != c.Delivery || record["device"] != c.Device || record["demux"] != "/dev/dvb/adapter0/demux0" || record["dvr"] != "/dev/dvb/adapter0/dvr0" {
			t.Fatalf("%s: %s", channel, buf.String())
		}
		switch channel {
		case "27":
			if record["bandwidth_hz"] != float64(6000000) || record["modulation"] != nil {
				t.Fatal(buf.String())
			}
		case "C53":
			if record["modulation"] != "qam64" || record["symbol_rate_ksps"] != float64(5274) {
				t.Fatal(buf.String())
			}
		default:
			if record["stream_id"] != channel {
				t.Fatal(buf.String())
			}
		}
		if channel == "0xb110" && record["format"] != "tlv" {
			t.Fatal(buf.String())
		}
	}
}
func TestArgumentErrorLogging(t *testing.T) {
	var buf bytes.Buffer
	if code := run([]string{"--unknown"}, &buf); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(buf.String(), "level=ERROR") || !strings.Contains(buf.String(), "reason=invalid_arguments") || strings.Count(buf.String(), "\n") != 1 {
		t.Fatal(buf.String())
	}
	buf.Reset()
	if code := run([]string{"--help"}, &buf); code != 0 || strings.Contains(buf.String(), "level=ERROR") || !strings.Contains(buf.String(), "Usage:") {
		t.Fatalf("help: %d %s", code, buf.String())
	}
}
