//go:build linux && (amd64 || arm64)

package dvb

import (
	"encoding/binary"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	feSetProperty = 0x40106f52
	feGetProperty = 0x80106f53
	feReadStatus  = 0x80046f45
	feGetInfo     = 0x80a86f3d
	feSetVoltage  = 0x6f43
	dmxSetFilter  = 0x40146f2c
	dmxStart      = 0x6f29
	dmxStop       = 0x6f2a
	dmxSetBuffer  = 0x6f2d
)

const (
	dtvTune                   = 1
	dtvClear                  = 2
	dtvFrequency              = 3
	dtvModulation             = 4
	dtvBandwidthHz            = 5
	dtvInversion              = 6
	dtvSymbolRate             = 8
	dtvInnerFEC               = 9
	dtvDeliverySystem         = 17
	dtvISDBTPartialReception  = 18
	dtvISDBTSoundBroadcasting = 19
	dtvISDBTLayerAFEC         = 23
	dtvISDBTLayerBFEC         = 27
	dtvISDBTLayerCFEC         = 31
	dtvGuardInterval          = 38
	dtvTransmissionMode       = 39
	dtvISDBTLayerEnabled      = 41
	dtvStreamID               = 42
	dtvEnumDelsys             = 44
	dtvAuto                   = 0xffffffff
)

const (
	sysDVBCAnnexA = 1
	sysDVBCAnnexC = 18
	sysISDBS      = 9
	sysISDBT      = 8
)

const (
	inversionOff         = 0
	inversionOn          = 1
	inversionAuto        = 2
	voltage13            = 0
	voltage18            = 1
	voltageOff           = 2
	qam64                = 3
	qam256               = 5
	qamAuto              = 6
	fecAuto              = 9
	guardIntervalAuto    = 4
	transmissionModeAuto = 2
	feHasLock            = 0x10
	dmxAllPIDs           = 0x2000
	dmxOutTSTap          = 2
	dmxPESOther          = 20
)

// Use bytes to preserve the packed C union layout.
type property [76]byte

func prop(cmd, value uint32) property {
	var p property
	binary.NativeEndian.PutUint32(p[:4], cmd)
	binary.NativeEndian.PutUint32(p[16:20], value)
	return p
}

//go:uintptrescapes
func ioctl(fd int, request uintptr, arg uintptr) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), request, arg)
	if errno != 0 {
		return errno
	}
	return nil
}

type propertyList struct {
	Count uint32
	Pad   uint32
	Props *property
}

type pesFilter struct {
	PID                        uint16
	Pad                        uint16
	Input, Output, Type, Flags uint32
}

func properties(fd int, request uintptr, p []property) error {
	args := propertyList{Count: uint32(len(p)), Props: &p[0]}
	err := ioctl(fd, request, uintptr(unsafe.Pointer(&args)))
	runtime.KeepAlive(p)
	runtime.KeepAlive(args)
	if err != nil {
		return err
	}
	for _, v := range p {
		if r := int32(binary.NativeEndian.Uint32(v[72:])); r < 0 {
			return unix.Errno(-r)
		}
	}
	return nil
}
