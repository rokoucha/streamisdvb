//go:build linux && (amd64 || arm64)

package dvb

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unsafe"

	"github.com/rokoucha/streamisdvb/internal/config"
)

func (s *session) tune(ctx context.Context, c config.Config, log *slog.Logger) error {
	var err error
	p := []property{prop(dtvEnumDelsys, 0)}
	if err = properties(s.frontend, feGetProperty, p); err != nil {
		return fmt.Errorf("DTV_ENUM_DELSYS: %w", err)
	}
	n := binary.NativeEndian.Uint32(p[0][48:52])
	if n > 32 {
		return errors.New("invalid delivery system count")
	}
	system, err := selectSystem(c.Delivery, p[0][16:16+int(n)])
	if err != nil {
		return err
	}
	if c.Delivery == "isdb-s3" {
		var info [168]byte
		if err = ioctl(s.frontend, feGetInfo, uintptr(unsafe.Pointer(&info))); err != nil {
			return fmt.Errorf("FE_GET_INFO: %w", err)
		}
		name := string(bytes.TrimRight(info[:128], "\x00"))
		if err = checkS3Frontend(name); err != nil {
			return err
		}
	}
	if err = properties(s.frontend, feSetProperty, []property{prop(dtvClear, 0)}); err != nil {
		return fmt.Errorf("DTV_CLEAR: %w", err)
	}
	if c.Satellite() {
		if err := s.setLNBSupply(ctx, c.LNBVoltage); err != nil {
			return err
		}
	}
	tune := tuneProperties(c, system)
	log.Info("tuning", "dvb_delivery_system", system)
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = properties(s.frontend, feSetProperty, tune); err != nil {
		return fmt.Errorf("%s tune: %w", c.Delivery, err)
	}
	if err := s.waitLock(ctx, c.TuneTimeout); err != nil {
		return err
	}
	log.Info("frontend locked")
	return nil
}

func (s *session) setLNBSupply(ctx context.Context, voltage string) error {
	value := uintptr(voltageOff)
	switch voltage {
	case "13":
		value = voltage13
	case "18":
		value = voltage18
	}
	s.lnbPowered = value != voltageOff
	if err := ioctl(s.frontend, feSetVoltage, value); err != nil {
		return fmt.Errorf("set LNB supply: %w", err)
	}
	if s.lnbPowered {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
	return nil
}

func (s *session) waitLock(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		var status uint32
		if err := ioctl(s.frontend, feReadStatus, uintptr(unsafe.Pointer(&status))); err != nil {
			return fmt.Errorf("FE_READ_STATUS: %w", err)
		}
		if status&feHasLock != 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return ErrTuneTimeout
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func selectSystem(delivery string, supported []byte) (uint32, error) {
	var candidates []byte
	switch delivery {
	case "isdb-s", "isdb-s3":
		candidates = []byte{sysISDBS}
	case "isdb-t":
		candidates = []byte{sysISDBT}
	case "isdb-c":
		candidates = []byte{sysDVBCAnnexC, sysDVBCAnnexA}
	case "dvbc-annex-a":
		candidates = []byte{sysDVBCAnnexA}
	case "dvbc-annex-c":
		candidates = []byte{sysDVBCAnnexC}
	default:
		return 0, fmt.Errorf("unsupported delivery system %q", delivery)
	}
	for _, want := range candidates {
		for _, got := range supported {
			if want == got {
				return uint32(want), nil
			}
		}
	}
	return 0, fmt.Errorf("frontend does not support %s", delivery)
}
func tuneProperties(c config.Config, system uint32) []property {
	inversion := uint32(inversionAuto)
	switch c.Inversion {
	case "off":
		inversion = inversionOff
	case "on":
		inversion = inversionOn
	}
	frequency := uint32(c.Frequency)
	if c.Satellite() {
		frequency /= 1000
	}
	tune := []property{prop(dtvDeliverySystem, system), prop(dtvFrequency, frequency), prop(dtvInversion, inversion)}
	if system == sysISDBT {
		tune = append(tune,
			prop(dtvBandwidthHz, uint32(c.Bandwidth)),
			prop(dtvGuardInterval, guardIntervalAuto),
			prop(dtvTransmissionMode, transmissionModeAuto),
			prop(dtvISDBTPartialReception, dtvAuto),
			prop(dtvISDBTSoundBroadcasting, 0),
			prop(dtvISDBTLayerEnabled, 7),
		)
		for _, base := range []uint32{dtvISDBTLayerAFEC, dtvISDBTLayerBFEC, dtvISDBTLayerCFEC} {
			tune = append(tune, prop(base, fecAuto), prop(base+1, qamAuto), prop(base+2, dtvAuto), prop(base+3, dtvAuto))
		}
	} else if c.Satellite() {
		tune = append(tune, prop(dtvSymbolRate, uint32(c.SymbolRate)), prop(dtvStreamID, c.StreamID))
	} else {
		modulation := uint32(qam64)
		if c.Modulation == "qam256" {
			modulation = qam256
		}
		tune = append(tune, prop(dtvSymbolRate, uint32(c.SymbolRate)), prop(dtvModulation, modulation), prop(dtvInnerFEC, fecAuto))
	}
	return append(tune, prop(dtvTune, 0))
}

func checkS3Frontend(name string) error {
	// S3 requires a driver-specific SYS_ISDBS extension.
	if !strings.HasPrefix(name, "Turbosight TBS 6812") {
		return fmt.Errorf("ISDB-S3 requires the supported TBS6812 driver; frontend is %q", name)
	}
	return nil
}
