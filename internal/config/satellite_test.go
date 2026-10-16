package config

import (
	"io"
	"testing"
)

func TestSatelliteChannels(t *testing.T) {
	for _, tc := range []struct {
		channel, delivery string
		frequency         uint
		stream            uint32
	}{
		{"0x4010", "isdb-s", 1049480000, 0x4010},
		{"0x4070", "isdb-s", 1164560000, 0x4070},
		{"0xb070", "isdb-s3", 1164560000, 0xb070},
		{"0x40b0", "isdb-s", 1241280000, 0x40b0},
		{"0xb0b0", "isdb-s3", 1241280000, 0xb0b0},
		{"0x4110", "isdb-s", 1356360000, 0x4110},
		{"0x4020", "isdb-s", 2241660000, 0x4020},
		{"0x6010", "isdb-s", 2766000000, 0x6010},
		{"0xc020", "isdb-s3", 1613000000, 0xc020},
		{"0x40f1", "isdb-s", 1318000000, 0x40f1},
		{"0x4730", "isdb-s", 1394720000, 0x4730},
		{"0x6020", "isdb-s", 1613000000, 0x6020},
		{"0x7180", "isdb-s", 2053000000, 0x7180},
		{"0x71a0", "isdb-s", 1550000000, 0x71a0},
		{"0xb110", "isdb-s3", 1356360000, 0xb110},
		{"0xb020", "isdb-s3", 2241660000, 0xb020},
		{"0xB180", "isdb-s3", 2663620000, 0xb180},
		{"0xc010", "isdb-s3", 2766000000, 0xc010},
		{"0xc170", "isdb-s3", 3206000000, 0xc170},
		{"0xc190", "isdb-s3", 2726000000, 0xc190},
	} {
		c, err := Parse([]string{tc.channel}, io.Discard)
		if err != nil {
			t.Fatalf("%s: %v", tc.channel, err)
		}
		if c.Delivery != tc.delivery || c.Frequency != tc.frequency || c.StreamID != tc.stream {
			t.Fatalf("%+v: got %+v", tc, c)
		}
		wantRate := uint(28860000)
		if tc.delivery == "isdb-s3" {
			wantRate = 33750000
		}
		if c.SymbolRate != wantRate || c.Modulation != "" || c.LNBVoltage != "off" {
			t.Fatalf("satellite defaults: %+v", c)
		}
	}
}
func TestInvalidSatelliteChannels(t *testing.T) {
	for _, channel := range []string{"0x", "0x0", "0xffff", "0x10000", "0xfoo", "0x4018", "0x5010", "0x4000", "0x4190", "BS15_0", "bs01_7", "CS24_0", "BS07_0", "BS11_0", "BS17_0", "BS02_0", "CS01_0", "BS0_0", "BS25_0", "BS15", "BS15_8", "BS15_-1", "BS+15_0", "BS15_+1", "CS28_0", "BS15_0_0"} {
		if _, err := Parse([]string{channel}, io.Discard); err == nil {
			t.Fatalf("accepted %s", channel)
		}
	}
	for _, args := range [][]string{
		{"0xb110", "--delivery-system", "isdb-s"},
		{"0x40f1", "--delivery-system", "isdb-s3"},
		{"BS17_0", "--delivery-system", "isdb-s3"},
		{"BS15_0", "--delivery-system", "isdb-s"},
		{"27", "--delivery-system", "isdb-s"},
		{"0xb110", "--frequency", "1356360000"},
		{"0x40f1", "--symbol-rate", "28860"},
		{"0x40f1", "--modulation", "qam64"},
		{"0x40f1", "--lnb-voltage", "15"},
		{"27", "--lnb-voltage", "13"},
		{"--frequency", "1318000000", "--delivery-system", "isdb-s"},
	} {
		if _, err := Parse(args, io.Discard); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	for _, voltage := range []string{"off", "13", "18"} {
		if _, err := Parse([]string{"0x40f1", "--lnb-voltage", voltage}, io.Discard); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSatelliteDefaults(t *testing.T) {
	for _, name := range []string{"BS07", "BS11", "BS17"} {
		if channelTable[name].defaultDelivery != "isdb-s3" {
			t.Fatalf("%s must default to ISDB-S3", name)
		}
	}
}
