//go:build linux && (amd64 || arm64)

package dvb

import (
	"encoding/binary"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/rokoucha/streamisdvb/internal/config"
)

func TestPropertyABI(t *testing.T) {
	p := prop(3, 473142857)
	if unsafe.Sizeof(p) != 76 || binary.NativeEndian.Uint32(p[16:20]) != 473142857 {
		t.Fatal("invalid packed property")
	}
	args := propertyList{}
	if unsafe.Sizeof(args) != 16 || unsafe.Offsetof(args.Props) != 8 {
		t.Fatal("invalid properties ABI")
	}
	filter := pesFilter{}
	if unsafe.Sizeof(filter) != 20 || unsafe.Offsetof(filter.Input) != 4 || unsafe.Offsetof(filter.Flags) != 16 {
		t.Fatal("invalid PES filter ABI")
	}
}
func TestCloseContinuesAfterStopFailure(t *testing.T) {
	fds := make([]int, 2)
	if err := unix.Pipe(fds); err != nil {
		t.Fatal(err)
	}
	s := session{frontend: fds[0], demux: -1, dvr: fds[1], attempted: true, lnbPowered: true}
	err := s.close()
	var uncertain *UnsafeTunerError
	if !errors.As(err, &uncertain) || !strings.Contains(err.Error(), "LNB supply off") {
		t.Fatalf("missing unsafe state: %v", err)
	}
	for _, fd := range fds {
		if _, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); err != unix.EBADF {
			t.Fatalf("descriptor %d remains open", fd)
		}
	}
}
func TestEmptySessionClose(t *testing.T) {
	s := session{frontend: -1, demux: -1, dvr: -1}
	if err := s.close(); err != nil {
		t.Fatal(err)
	}
}

func TestSelectSystem(t *testing.T) {
	for _, tc := range []struct {
		delivery  string
		supported []byte
		want      uint32
	}{
		{"isdb-t", []byte{8, 1}, 8},
		{"isdb-c", []byte{1, 18}, 18},
		{"isdb-c", []byte{1}, 1},
		{"dvbc-annex-a", []byte{1, 18}, 1},
		{"dvbc-annex-c", []byte{1, 18}, 18},
	} {
		got, err := selectSystem(tc.delivery, tc.supported)
		if err != nil || got != tc.want {
			t.Fatalf("%+v: got %d, %v", tc, got, err)
		}
	}
	for _, delivery := range []string{"isdb-t", "isdb-c", "dvbc-annex-c", "unknown"} {
		if _, err := selectSystem(delivery, []byte{3, 16}); err == nil {
			t.Fatalf("accepted unsupported %s", delivery)
		}
	}
}
func TestCableTuneProperties(t *testing.T) {
	for _, tc := range []struct {
		modulation string
		want       uint32
	}{{"qam64", 3}, {"qam256", 5}} {
		p := tuneProperties(config.Config{Frequency: 405000000, SymbolRate: 5274000, Modulation: tc.modulation}, sysDVBCAnnexA)
		values := map[uint32]uint32{}
		for _, v := range p {
			values[binary.NativeEndian.Uint32(v[:4])] = binary.NativeEndian.Uint32(v[16:20])
		}
		if len(p) != 7 || values[dtvModulation] != tc.want || values[dtvInnerFEC] != 9 || values[dtvSymbolRate] != 5274000 || values[dtvDeliverySystem] != sysDVBCAnnexA || values[dtvFrequency] != 405000000 {
			t.Fatalf("unexpected cable properties: %v", values)
		}
		if binary.NativeEndian.Uint32(p[len(p)-1][:4]) != dtvTune {
			t.Fatal("DTV_TUNE must be last")
		}
	}
}

func TestInversionProperties(t *testing.T) {
	for _, tc := range []struct {
		name string
		want uint32
	}{{"auto", 2}, {"off", 0}, {"on", 1}} {
		p := tuneProperties(config.Config{Inversion: tc.name}, sysDVBCAnnexA)
		if got := binary.NativeEndian.Uint32(p[2][16:20]); got != tc.want {
			t.Fatalf("%s: got %d", tc.name, got)
		}
	}
}

func TestSatelliteTuneProperties(t *testing.T) {
	for _, channel := range []string{"0x40f1", "0xb110", "0xc170"} {
		c, err := config.Parse([]string{channel}, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		system, err := selectSystem(c.Delivery, []byte{sysISDBS})
		if err != nil || system != sysISDBS {
			t.Fatalf("%s: %d %v", channel, system, err)
		}
		p := tuneProperties(c, system)
		values := map[uint32]uint32{}
		for _, v := range p {
			values[binary.NativeEndian.Uint32(v[:4])] = binary.NativeEndian.Uint32(v[16:20])
		}
		if values[dtvFrequency] != uint32(c.Frequency/1000) || values[dtvSymbolRate] != uint32(c.SymbolRate) || values[dtvStreamID] != c.StreamID || values[dtvDeliverySystem] != sysISDBS || len(p) != 6 {
			t.Fatalf("%s: invalid satellite properties %v", channel, values)
		}
		if binary.NativeEndian.Uint32(p[len(p)-1][:4]) != dtvTune {
			t.Fatal("tune must be last")
		}
	}
	for _, delivery := range []string{"isdb-s", "isdb-s3"} {
		if _, err := selectSystem(delivery, []byte{sysISDBT}); err == nil {
			t.Fatal("accepted terrestrial-only frontend")
		}
	}
}
func TestSatelliteDriverCompatibility(t *testing.T) {
	if err := checkS3Frontend("Turbosight TBS 6812 (Dual ISDB-T/S/S3)"); err != nil {
		t.Fatal(err)
	}
	if checkS3Frontend("Toshiba TC90522 ISDB-S") == nil {
		t.Fatal("accepted driver without S3 extension")
	}
}

func TestSessionReleaseOrder(t *testing.T) {
	for _, fail := range []string{"", "stop", "close DVR", "close demux", "power off", "close frontend"} {
		t.Run("failure="+fail, func(t *testing.T) {
			s := session{frontend: 10, demux: 11, dvr: 12, attempted: true, lnbPowered: true}
			var operations []string
			record := func(op string) error {
				operations = append(operations, op)
				if op == fail {
					return unix.EIO
				}
				return nil
			}
			err := s.release(func(fd int, request, arg uintptr) error {
				switch {
				case fd == 11 && request == dmxStop && arg == 0:
					return record("stop")
				case fd == 10 && request == feSetVoltage && arg == voltageOff:
					return record("power off")
				default:
					t.Fatalf("unexpected ioctl: fd=%d request=%x arg=%d", fd, request, arg)
					return nil
				}
			}, func(fd int) error {
				switch fd {
				case 12:
					return record("close DVR")
				case 11:
					return record("close demux")
				case 10:
					return record("close frontend")
				default:
					t.Fatalf("unexpected close: %d", fd)
					return nil
				}
			})
			want := []string{"stop", "close DVR", "close demux", "power off", "close frontend"}
			if !slices.Equal(operations, want) {
				t.Fatalf("release order: %v", operations)
			}
			var uncertain *UnsafeTunerError
			if fail == "" && err != nil {
				t.Fatal(err)
			}
			if fail != "" && (!errors.As(err, &uncertain) || !errors.Is(err, unix.EIO)) {
				t.Fatalf("missing uncertain state: %v", err)
			}
		})
	}
}
