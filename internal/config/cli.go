package config

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
)

func Parse(args []string, output io.Writer) (Config, error) {
	var c Config
	var device, symbolRate uint
	fs := flag.NewFlagSet("streamisdvb", flag.ContinueOnError)
	var usage bytes.Buffer
	fs.SetOutput(&usage)
	fs.Usage = func() {
		fmt.Fprintln(&usage, "Usage: streamisdvb CHANNEL [options]\n       streamisdvb --frequency HZ [options]\nCHANNEL: 27, C53, 0x40f1, 0xb110")
		fs.PrintDefaults()
	}
	fs.UintVar(&device, "device", 0, "DVB adapter number")
	fs.StringVar(&c.Demux, "demux", "", "demux device (default: demux0 beside frontend)")
	fs.StringVar(&c.DVR, "dvr", "", "DVR device (default: dvr0 beside frontend)")
	fs.StringVar(&c.Delivery, "delivery-system", "", "isdb-t, isdb-c, isdb-s, isdb-s3, dvbc-annex-a or dvbc-annex-c (default: infer from CHANNEL)")
	fs.StringVar(&c.LNBVoltage, "lnb-voltage", "off", "satellite LNB supply: off, 13 or 18 (DVB voltage API)")
	fs.StringVar(&c.Inversion, "inversion", "auto", "spectral inversion: auto, off or on")
	fs.StringVar(&c.Modulation, "modulation", "qam64", "cable modulation: qam64 or qam256")
	fs.UintVar(&symbolRate, "symbol-rate", 5274, "cable symbol rate in ksymbols/s")
	fs.UintVar(&c.Frequency, "frequency", 0, "frequency in Hz (instead of CHANNEL)")
	fs.UintVar(&c.Bandwidth, "bandwidth", 6000000, "bandwidth in Hz (must be 6000000)")
	fs.DurationVar(&c.TuneTimeout, "tune-timeout", 10*time.Second, "frontend lock timeout")
	fs.DurationVar(&c.IdleTimeout, "idle-timeout", 10*time.Second, "stream inactivity timeout")

	// flag.Parse stops at the first positional argument.
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		args = append(append([]string{}, args[1:]...), args[0])
	}
	err := fs.Parse(args)
	c.Device = fmt.Sprintf("/dev/dvb/adapter%d/frontend0", device)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			if _, e := io.Copy(output, &usage); e != nil {
				return c, e
			}
		}
		return c, err
	}
	if fs.NArg() > 1 {
		return c, errors.New("specify one channel")
	}
	explicit := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	c.Channel = fs.Arg(0)
	if err := c.resolveChannel(c.Channel, explicit["frequency"]); err != nil {
		return c, err
	}
	if err := c.resolveTuning(symbolRate, explicit["symbol-rate"] || explicit["modulation"]); err != nil {
		return c, err
	}
	if c.Demux == "" {
		c.Demux = filepath.Join(filepath.Dir(c.Device), "demux0")
	}
	if c.DVR == "" {
		c.DVR = filepath.Join(filepath.Dir(c.Device), "dvr0")
	}
	return c, c.validate()
}
