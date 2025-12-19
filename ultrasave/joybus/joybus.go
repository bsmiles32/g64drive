package joybus

import (
	"encoding/binary"
)

// Abstact Joybus controller.
type Controller interface {
	Execute(cmd Command, tx, rx []byte) error
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
	Joybus Controller
}

// Ensure *DeviceImpl implements Device interface at compile time.
var _ Device = (*DeviceImpl)(nil)

func (d *DeviceImpl) Info() (DeviceID, Status, error) {
	return Info(d.Joybus, cmdInfo)
}

func (d *DeviceImpl) Reset() (DeviceID, Status, error) {
	return Info(d.Joybus, cmdReset)
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
