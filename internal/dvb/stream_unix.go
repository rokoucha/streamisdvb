//go:build linux || darwin

package dvb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"golang.org/x/sys/unix"
)

func ready(ctx context.Context, fd int, events int16, deadline time.Time) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			return ErrStreamTimeout
		}
		p := []unix.PollFd{{Fd: int32(fd), Events: events}}
		_, err := unix.Poll(p, 100)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return err
		}
		if p[0].Revents&unix.POLLNVAL != 0 {
			return unix.EBADF
		}
		if p[0].Revents&(events|unix.POLLERR|unix.POLLHUP) != 0 {
			return nil
		}
	}
}
func stream(ctx context.Context, src, dst int, timeout time.Duration) (err error) {
	flags, err := unix.FcntlInt(uintptr(dst), unix.F_GETFL, 0)
	if err != nil {
		return err
	}
	if err = unix.SetNonblock(dst, true); err != nil {
		return err
	}
	defer func() { _, e := unix.FcntlInt(uintptr(dst), unix.F_SETFL, flags); err = errors.Join(err, e) }()
	buf := make([]byte, 188*256)
	deadline := time.Now().Add(timeout)
	for {
		if err = ready(ctx, src, unix.POLLIN, deadline); err != nil {
			return err
		}
		n, e := unix.Read(src, buf)
		if e == unix.EAGAIN || e == unix.EINTR {
			continue
		}
		if e != nil {
			return fmt.Errorf("read DVR: %w", e)
		}
		if n == 0 {
			return io.ErrUnexpectedEOF
		}
		deadline = time.Now().Add(timeout)
		for b := buf[:n]; len(b) > 0; {
			if err = ready(ctx, dst, unix.POLLOUT, deadline); err != nil {
				return err
			}
			n, e = unix.Write(dst, b)
			if e == unix.EAGAIN || e == unix.EINTR {
				continue
			}
			if e != nil {
				return fmt.Errorf("write stream: %w", e)
			}
			if n == 0 {
				return io.ErrShortWrite
			}
			b = b[n:]
			deadline = time.Now().Add(timeout)
		}
	}
}
