//go:build linux || darwin

package dvb

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func pipe(t *testing.T) []int {
	t.Helper()
	p := make([]int, 2)
	if err := unix.Pipe(p); err != nil {
		t.Fatal(err)
	}
	for _, fd := range p {
		if err := unix.SetNonblock(fd, true); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, fd := range p {
			unix.Close(fd)
		}
	})
	return p
}
func TestStreamPreservesBytes(t *testing.T) {
	for name, data := range map[string][]byte{
		"TS":  bytes.Repeat([]byte{0x47, 0, 0, 0xff}, 188),
		"TLV": bytes.Repeat([]byte{0x7f, 0x02, 0x00, 0x03, 0xde, 0xad, 0xbe}, 91),
	} {
		t.Run(name, func(t *testing.T) {
			src, dst := pipe(t), pipe(t)
			if _, err := unix.Write(src[1], data); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- stream(ctx, src[0], dst[1], time.Second) }()
			if err := ready(ctx, dst[0], unix.POLLIN, time.Now().Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			got := make([]byte, len(data))
			n, err := unix.Read(dst[0], got)
			if err != nil || !bytes.Equal(got[:n], data) {
				t.Fatalf("stream changed: n=%d err=%v", n, err)
			}
			cancel()
			if err := <-done; !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		})
	}
}

func TestStreamCancellation(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		t.Run(map[bool]string{false: "no signal", true: "blocked output"}[blocked], func(t *testing.T) {
			src, dst := pipe(t), pipe(t)
			if blocked {
				buf := make([]byte, 4096)
				for {
					_, err := unix.Write(dst[1], buf)
					if err == unix.EAGAIN {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				if _, err := unix.Write(src[1], buf); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- stream(ctx, src[0], dst[1], time.Second) }()
			time.Sleep(20 * time.Millisecond)
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("stream did not stop")
			}
		})
	}
}
func TestStreamTimeoutAndBrokenPipe(t *testing.T) {
	src, dst := pipe(t), pipe(t)
	if err := stream(context.Background(), src[0], dst[1], 20*time.Millisecond); err == nil {
		t.Fatal("missing idle timeout")
	}
	unix.Close(dst[0])
	dst[0] = -1
	unix.Write(src[1], []byte{0x47})
	if err := stream(context.Background(), src[0], dst[1], time.Second); !errors.Is(err, unix.EPIPE) {
		t.Fatalf("want EPIPE, got %v", err)
	}
}
