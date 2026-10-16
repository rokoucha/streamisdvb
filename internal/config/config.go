package config

import (
	"errors"
	"fmt"
	"time"
)

type Config struct {
	Device, Demux, DVR              string
	Delivery, Modulation, Inversion string
	SymbolRate                      uint
	StreamID                        uint32
	LNBVoltage                      string
	Channel                         string
	Frequency, Bandwidth            uint
	TuneTimeout, IdleTimeout        time.Duration
}

func (c *Config) resolveTuning(symbolRateKSps uint, cableOptionsSet bool) error {
	if c.Delivery == "isdb-t" || c.Satellite() {
		if cableOptionsSet {
			return errors.New("--symbol-rate and --modulation are only valid for cable")
		}
		c.Modulation = ""
		if c.Satellite() {
			c.SymbolRate = 28860000
			if c.Delivery == "isdb-s3" {
				c.SymbolRate = 33750000
			}
		}
	} else {
		if symbolRateKSps == 0 || uint64(symbolRateKSps) > 0xffffffff/1000 {
			return errors.New("--symbol-rate must be 1–4294967 ksymbols/s (e.g. 5274)")
		}
		c.SymbolRate = symbolRateKSps * 1000
	}
	return nil
}

func (c Config) validate() error {
	if c.Frequency == 0 || uint64(c.Frequency) > 0xffffffff || c.TuneTimeout <= 0 || c.IdleTimeout <= 0 {
		return errors.New("specify CHANNEL or --frequency in Hz and positive timeouts")
	}
	if c.Inversion != "auto" && c.Inversion != "off" && c.Inversion != "on" {
		return errors.New("--inversion must be auto, off or on")
	}
	if c.LNBVoltage != "" && c.LNBVoltage != "off" && c.LNBVoltage != "13" && c.LNBVoltage != "18" {
		return errors.New("--lnb-voltage must be off, 13 or 18")
	}
	if !c.Satellite() && c.LNBVoltage != "" && c.LNBVoltage != "off" {
		return errors.New("--lnb-voltage is only valid for satellite")
	}
	switch c.Delivery {
	case "isdb-s", "isdb-s3":
		if c.Frequency%1000 != 0 || c.SymbolRate == 0 || c.Modulation != "" || c.Bandwidth != 6000000 {
			return errors.New("invalid satellite tuning parameters")
		}
		if c.StreamID > 65535 || c.StreamID < 8 {
			return errors.New("satellite reception requires a valid stream ID")
		}

	case "isdb-t":
		if c.Bandwidth != 6000000 || c.SymbolRate != 0 || c.Modulation != "" {
			return errors.New("ISDB-T requires bandwidth 6000000 and no cable parameters")
		}
	case "isdb-c", "dvbc-annex-a", "dvbc-annex-c":
		if c.SymbolRate == 0 || uint64(c.SymbolRate) > 0xffffffff {
			return errors.New("invalid cable symbol rate in symbols/s")
		}
		if c.Modulation != "qam64" && c.Modulation != "qam256" {
			return errors.New("cable requires --modulation qam64 or qam256")
		}
		if c.Bandwidth != 6000000 {
			return errors.New("cable requires --bandwidth 6000000")
		}
	default:
		return fmt.Errorf("unsupported delivery system %q", c.Delivery)
	}
	return nil
}
