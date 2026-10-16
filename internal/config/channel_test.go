package config

import (
	"io"
	"testing"
)

func TestChannelFrequency(t *testing.T) {
	for _, tc := range []struct {
		channel, delivery string
		frequency         uint
		selected          string
	}{
		{"13", "", 473142857, "isdb-t"}, {"27", "", 557142857, "isdb-t"}, {"62", "", 767142857, "isdb-t"},
		{"27", "isdb-c", 557000000, "isdb-c"}, {"1", "", 93000000, "isdb-c"}, {"3", "", 105000000, "isdb-c"},
		{"4", "", 173000000, "isdb-c"}, {"7", "", 191000000, "isdb-c"}, {"8", "", 195000000, "isdb-c"}, {"12", "", 219000000, "isdb-c"},
		{"C13", "", 111000000, "isdb-c"}, {"C21", "", 159000000, "isdb-c"}, {"C22", "", 167000000, "isdb-c"},
		{"C23", "", 225000000, "isdb-c"}, {"C53", "", 405000000, "isdb-c"}, {"c63", "", 465000000, "isdb-c"},
		{"C53", "isdb-t", 405142857, "isdb-t"}, {"11", "isdb-t", 213142857, "isdb-t"},
		{"29", "dvbc-annex-a", 569000000, "dvbc-annex-a"},
	} {
		c := Config{Delivery: tc.delivery}
		if err := c.resolveChannel(tc.channel, false); err != nil {
			t.Fatalf("%+v: %v", tc, err)
		}
		if c.Frequency != tc.frequency || c.Delivery != tc.selected {
			t.Fatalf("%+v: got %d %s", tc, c.Frequency, c.Delivery)
		}
	}
	for _, channel := range []string{"0", "63", "C12", "C64", "CC13", "-1", "+27", "27.0", "C", "999999999999999999999999999"} {
		c := Config{}
		if c.resolveChannel(channel, false) == nil {
			t.Fatalf("accepted %s", channel)
		}
	}
}
func TestParseChannel(t *testing.T) {
	for _, args := range [][]string{
		{"27"},
		{"27", "--device", "1"},
		{"--device", "1", "27"},
	} {
		c, err := Parse(args, io.Discard)
		if err != nil || c.Frequency != 557142857 || c.Delivery != "isdb-t" {
			t.Fatalf("%v: %+v, %v", args, c, err)
		}
	}
	for _, args := range [][]string{
		{"C53"}, {"11"}, {"27", "--delivery-system", "isdb-c"},
		{"C53", "--symbol-rate", "5274", "--modulation", "qam64"},
		{"29", "--delivery-system", "isdb-c", "--symbol-rate", "5274", "--modulation", "qam256"},
	} {
		c, err := Parse(args, io.Discard)
		if err != nil || c.Delivery != "isdb-c" {
			t.Fatalf("%v: %+v, %v", args, c, err)
		}
	}
	for _, args := range [][]string{
		{}, {"27", "28"}, {"27", "--frequency", "557142857"}, {"27", "--frequency", "0"},
		{"--frequency", "557142857", "27"},
	} {
		if _, err := Parse(args, io.Discard); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	c, err := Parse([]string{"--frequency", "557142857"}, io.Discard)
	if err != nil || c.Frequency != 557142857 || c.Delivery != "isdb-t" {
		t.Fatalf("frequency compatibility: %+v %v", c, err)
	}
}

func TestDeviceNumber(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		adapter string
	}{
		{[]string{"27"}, "0"},
		{[]string{"27", "--device", "2"}, "2"},
		{[]string{"--device", "12", "C53"}, "12"},
	} {
		c, err := Parse(tc.args, io.Discard)
		base := "/dev/dvb/adapter" + tc.adapter
		if err != nil || c.Device != base+"/frontend0" || c.Demux != base+"/demux0" || c.DVR != base+"/dvr0" {
			t.Fatalf("%v: %+v %v", tc.args, c, err)
		}
	}
	c, err := Parse([]string{"27", "--device", "1", "--demux", "/custom/demux", "--dvr", "/custom/dvr"}, io.Discard)
	if err != nil || c.Device != "/dev/dvb/adapter1/frontend0" || c.Demux != "/custom/demux" || c.DVR != "/custom/dvr" {
		t.Fatalf("explicit nodes: %+v %v", c, err)
	}
	for _, device := range []string{"-1", "one", "/dev/dvb/adapter1/frontend0"} {
		if _, err := Parse([]string{"27", "--device", device}, io.Discard); err == nil {
			t.Fatalf("accepted device %q", device)
		}
	}
}
