package config

import (
	"io"
	"testing"
	"time"
)

func TestConfigValidation(t *testing.T) {
	valid := Config{Delivery: "isdb-t", Inversion: "auto", Frequency: 557142857, Bandwidth: 6000000, TuneTimeout: time.Second, IdleTimeout: time.Second}
	for _, delivery := range []string{"isdb-t", "isdb-c", "dvbc-annex-a", "dvbc-annex-c"} {
		c := valid
		c.Delivery = delivery
		if delivery != "isdb-t" {
			c.SymbolRate = 5274000
			c.Modulation = "qam64"
		}
		if err := c.validate(); err != nil {
			t.Fatalf("%s: %v", delivery, err)
		}
	}
	for _, change := range []func(*Config){
		func(c *Config) { c.Delivery = "unknown" },
		func(c *Config) { c.Inversion = "unknown" },
		func(c *Config) { c.Frequency = 0 },
		func(c *Config) { c.TuneTimeout = 0 },
		func(c *Config) { c.SymbolRate = 5274000 },
		func(c *Config) { c.Delivery = "isdb-c" },
		func(c *Config) { c.Delivery = "isdb-c"; c.SymbolRate = 5274000; c.Modulation = "auto" },
	} {
		c := valid
		change(&c)
		if c.validate() == nil {
			t.Fatalf("accepted invalid config %+v", c)
		}
	}
}

func TestCableDefaultsAndSymbolRateUnits(t *testing.T) {
	for _, tc := range []struct {
		args       []string
		rate       uint
		modulation string
	}{
		{[]string{"C53"}, 5274000, "qam64"},
		{[]string{"29", "--delivery-system", "isdb-c"}, 5274000, "qam64"},
		{[]string{"--frequency", "405000000", "--delivery-system", "dvbc-annex-a"}, 5274000, "qam64"},
		{[]string{"C53", "--symbol-rate", "5361", "--modulation", "qam256"}, 5361000, "qam256"},
		{[]string{"C53", "--symbol-rate", "4294967"}, 4294967000, "qam64"},
	} {
		c, err := Parse(tc.args, io.Discard)
		if err != nil || c.SymbolRate != tc.rate || c.Modulation != tc.modulation {
			t.Fatalf("%v: %+v %v", tc.args, c, err)
		}
	}
	for _, args := range [][]string{
		{"C53", "--symbol-rate", "0"}, {"C53", "--symbol-rate", "4294968"},
		{"C53", "--symbol-rate", "5274000"}, {"C53", "--symbol-rate", "5.274"},
		{"C53", "--modulation", "auto"}, {"27", "--symbol-rate", "5274"},
		{"27", "--symbol-rate", "0"}, {"27", "--modulation", "qam64"},
	} {
		if _, err := Parse(args, io.Discard); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	c, err := Parse([]string{"27"}, io.Discard)
	if err != nil || c.SymbolRate != 0 || c.Modulation != "" {
		t.Fatalf("terrestrial defaults: %+v %v", c, err)
	}
}
