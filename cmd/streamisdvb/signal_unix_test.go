//go:build linux || darwin

package main

import (
	"bufio"
	"bytes"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSignalProcess(t *testing.T) {
	if os.Getenv("STREAMISDVB_SIGNAL_TEST") == "1" {
		ctx, stop := signalContext()
		defer stop()
		os.Stdout.WriteString("ready\n")
		<-ctx.Done()
		os.Exit(logExit(slog.New(slog.NewTextHandler(os.Stderr, nil)).With("device", "test"), ctx, ctx.Err()))
	}
	for _, tc := range []struct {
		signal os.Signal
		name   string
	}{{os.Interrupt, "SIGINT"}, {syscall.SIGTERM, "SIGTERM"}} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestSignalProcess$")
			cmd.Env = append(os.Environ(), "STREAMISDVB_SIGNAL_TEST=1")
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if cmd.ProcessState == nil {
					cmd.Process.Kill()
					cmd.Wait()
				}
			}()
			ready := make(chan bool, 1)
			go func() { scanner := bufio.NewScanner(stdout); ready <- scanner.Scan() && scanner.Text() == "ready" }()
			select {
			case ok := <-ready:
				if !ok {
					t.Fatal("child not ready")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("child readiness timeout")
			}
			if err := cmd.Process.Signal(tc.signal); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				cmd.Process.Kill()
				<-done
				t.Fatal("signal shutdown timed out")
			}
			log := stderr.String()
			if !strings.Contains(log, "level=INFO") || !strings.Contains(log, "reason=signal") || !strings.Contains(log, "signal="+tc.name) || strings.Contains(log, "context canceled") || strings.Contains(log, "level=ERROR") {
				t.Fatal(log)
			}
		})
	}
}

func TestBrokenStderrProcess(t *testing.T) {
	if os.Getenv("STREAMISDVB_BROKEN_STDERR_TEST") == "1" {
		os.Args = []string{"streamisdvb", "--unknown"}
		main()
		return
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	reader.Close()
	defer writer.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestBrokenStderrProcess$")
	cmd.Env = append(os.Environ(), "STREAMISDVB_BROKEN_STDERR_TEST=1")
	cmd.Stderr = writer
	err = cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("want normal argument-error exit 1, got %v", err)
	}
}
