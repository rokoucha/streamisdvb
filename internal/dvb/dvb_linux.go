//go:build linux && (amd64 || arm64)

package dvb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/rokoucha/streamisdvb/internal/config"
)

type session struct {
	frontend, demux, dvr int
	attempted            bool
	lnbPowered           bool
}

func (s *session) close() error {
	return s.release(ioctl, unix.Close)
}

func (s *session) release(control func(int, uintptr, uintptr) error, closeFD func(int) error) error {
	var errs []error
	add := func(op string, err error) {
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", op, err))
		}
	}
	// DMX_START can change feed state even when it fails.
	if s.attempted {
		add("DMX_STOP", control(s.demux, dmxStop, 0))
	}
	for _, v := range []struct {
		name string
		fd   int
	}{{"close DVR", s.dvr}, {"close demux", s.demux}} {
		if v.fd >= 0 {
			add(v.name, closeFD(v.fd))
		}
	}
	if s.lnbPowered {
		add("LNB supply off", control(s.frontend, feSetVoltage, voltageOff))
	}
	if s.frontend >= 0 {
		add("close frontend", closeFD(s.frontend))
	}
	if err := errors.Join(errs...); err != nil {
		return &UnsafeTunerError{err}
	}
	return nil
}
func Receive(ctx context.Context, c config.Config, log *slog.Logger) (err error) {
	s := session{frontend: -1, demux: -1, dvr: -1}
	defer func() {
		cleanupErr := s.close()
		if cleanupErr != nil {
			log.Error("cleanup failed", "error", cleanupErr)
		} else {
			log.Info("cleanup completed")
		}
		err = errors.Join(err, cleanupErr)
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.openFrontend(c.Device); err != nil {
		return err
	}
	if err := s.tune(ctx, c, log); err != nil {
		return err
	}
	if err := s.start(ctx, c); err != nil {
		return err
	}
	log.Info("streaming started")
	return stream(ctx, s.dvr, int(os.Stdout.Fd()), c.IdleTimeout)
}

func openDevice(path string, flags int) (int, error) {
	fd, e := unix.Open(path, flags|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if e != nil {
		return -1, fmt.Errorf("open %s: %w", path, e)
	}
	return fd, nil
}

func (s *session) openFrontend(path string) error {
	var err error
	s.frontend, err = openDevice(path, unix.O_RDWR)
	if err != nil {
		return err
	}
	if err = unix.Flock(s.frontend, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return fmt.Errorf("lock frontend: %w", err)
	}
	return nil
}

func (s *session) start(ctx context.Context, c config.Config) error {
	var err error
	s.demux, err = openDevice(c.Demux, unix.O_RDWR)
	if err != nil {
		return err
	}
	s.dvr, err = openDevice(c.DVR, unix.O_RDONLY)
	if err != nil {
		return err
	}
	if err = ioctl(s.dvr, dmxSetBuffer, 4*1024*1024); err != nil {
		return fmt.Errorf("DVR buffer: %w", err)
	}
	filter := pesFilter{PID: dmxAllPIDs, Output: dmxOutTSTap, Type: dmxPESOther}
	if err = ioctl(s.demux, dmxSetFilter, uintptr(unsafe.Pointer(&filter))); err != nil {
		return fmt.Errorf("DMX_SET_PES_FILTER: %w", err)
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	s.attempted = true
	if err = ioctl(s.demux, dmxStart, 0); err != nil {
		return &UnsafeTunerError{fmt.Errorf("DMX_START: %w", err)}
	}
	return nil
}
