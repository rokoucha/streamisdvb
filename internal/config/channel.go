package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func (c *Config) resolveChannel(channel string, frequencySet bool) error {
	if channel == "" {
		if c.Satellite() {
			return errors.New("satellite reception requires a hexadecimal TSID or TLV stream ID")
		}
		if c.Delivery == "" {
			c.Delivery = "isdb-t"
		}
		return nil
	}
	if frequencySet {
		return errors.New("CHANNEL and --frequency cannot be combined")
	}
	name := strings.ToUpper(channel)
	if strings.HasPrefix(name, "0X") || strings.HasPrefix(name, "BS") || strings.HasPrefix(name, "CS") {
		return c.resolveSatellite(name)
	}
	if c.Satellite() {
		return errors.New("satellite reception requires a hexadecimal TSID or TLV stream ID")
	}
	cable := strings.HasPrefix(name, "C")
	number := name
	if cable {
		number = name[1:]
	}
	for _, r := range number {
		if r < '0' || r > '9' {
			return fmt.Errorf("invalid channel %q", channel)
		}
	}
	n, err := strconv.Atoi(number)
	if err != nil {
		return fmt.Errorf("invalid channel %q", channel)
	}
	key := strconv.Itoa(n)
	if cable {
		key = "C" + key
	}
	spec, ok := channelTable[key]
	if !ok {
		return fmt.Errorf("invalid channel %q (1–62 or C13–C63)", channel)
	}
	if c.Delivery == "" {
		c.Delivery = spec.defaultDelivery
	}
	c.Frequency = spec.frequency
	if c.Delivery == "isdb-t" {
		c.Frequency = spec.isdbTFrequency
	}
	return nil
}
