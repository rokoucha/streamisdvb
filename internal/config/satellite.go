package config

import (
	"fmt"
	"strconv"
	"strings"
)

func (c Config) Satellite() bool { return c.Delivery == "isdb-s" || c.Delivery == "isdb-s3" }

func (c *Config) resolveSatellite(name string) error {
	if !strings.HasPrefix(name, "0X") {
		return fmt.Errorf("satellite reception requires a hexadecimal TSID or TLV stream ID")
	}
	id, err := strconv.ParseUint(name[2:], 16, 16)
	if err != nil || id == 0 || id == 0xffff || id&8 != 0 {
		return fmt.Errorf("invalid satellite ID %q", name)
	}
	var band, delivery string
	switch id >> 12 {
	case 4:
		band, delivery = "BS", "isdb-s"
	case 6, 7:
		band, delivery = "CS", "isdb-s"
	case 11:
		band, delivery = "BS", "isdb-s3"
	case 12:
		band, delivery = "CS", "isdb-s3"
	default:
		return fmt.Errorf("unknown satellite ID allocation: %s", name)
	}
	transponder := int((id >> 4) & 31)
	spec, ok := channelTable[fmt.Sprintf("%s%02d", band, transponder)]
	if !ok {
		return fmt.Errorf("invalid %s transponder %d", band, transponder)
	}
	if c.Delivery != "" && c.Delivery != delivery {
		return fmt.Errorf("%s selects %s, conflicts with --delivery-system %s", name, delivery, c.Delivery)
	}
	c.StreamID = uint32(id)
	c.Delivery = delivery
	c.Frequency = spec.frequency
	return nil
}
