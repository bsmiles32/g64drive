package joybus

import (
	"encoding/binary"

	"github.com/rasky/g64drive/ultrasave/logger"
)

// Abstact Joybus interface
type Controller interface {
	Execute(cmd Command, tx, rx []byte) error
	Unfreeze(err error) bool
}

// Joybus commands are 8 bits.
type Command uint8

// Joybus common commands.
// All joybus devices should support these.
// Commands specific to each device are defined in their respective packages.
const (
	cmdInfo  = Command(0x00)
	cmdReset = Command(0xff)
)

type DeviceID uint16
type Status uint8

type Device interface {
	Info() (DeviceID, Status, error)
	Reset() (DeviceID, Status, error)
}

type DeviceImpl struct {
	Controller
}

// Ensure *DeviceImpl implements Device interface at compile time.
var _ Device = (*DeviceImpl)(nil)

func (d *DeviceImpl) Info() (DeviceID, Status, error) {
	return Info(d, cmdInfo)
}

func (d *DeviceImpl) Reset() (DeviceID, Status, error) {
	return Info(d, cmdReset)
}

func Info(joybus Controller, cmd Command) (DeviceID, Status, error) {
	var info [3]byte

	if err := joybus.Execute(cmd, nil, info[:]); err != nil {
		return 0, 0, err
	}

	id := DeviceID(binary.BigEndian.Uint16(info[:2]))
	status := Status(info[2])

	return id, status, nil
}

func Probe(c Controller, l logger.Logger) (DeviceID, error) {
	probes := []struct {
		class   string
		info    Command
		devices map[DeviceID]string
	}{
		{
			class: "regular",
			info:  cmdInfo,
			devices: map[DeviceID]string{
				DeviceID(0x0080): "EEPROM 4Kib",
				DeviceID(0x00c0): "EEPROM 16Kib",
			},
		},
		{
			class: "rtc",
			info:  Command(0x06),
			devices: map[DeviceID]string{
				DeviceID(0x0010): "RTC",
			},
		},
	}

	for _, p := range probes {
		logger.Log(l, "Probing for %s Joybus devices\n", p.class)
		devID, _, err := Info(c, p.info)

		if err == nil {
		} else if c.Unfreeze(err) {
			// We didn't get any response from joybus device, unfreeze the 64drive/SI device
			// and try next probing method
			continue
		} else {
			return 0, err
		}

		// Try next probing method if we get a Zero DeviceID
		if devID == 0x0000 {
			continue
		}

		// Either return a device using specific factory
		// or just a generic joybus device impl if device ID is unknown.
		if device, ok := p.devices[devID]; ok {
			logger.Log(l, "Found %s (ID = %04x)\n", device, devID)
			return devID, nil
		} else {
			logger.Log(l, "Found unknown device (ID = %04x)\n", devID)
			return devID, nil
		}
	}

	logger.Log(l, "No joybus device detected\n")
	return 0, nil
}
