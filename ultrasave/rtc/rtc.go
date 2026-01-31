package rtc

import (
	"errors"
	"github.com/rasky/g64drive/ultrasave/joybus"
)

var (
	ErrInvalidSize = errors.New("invalid block access size")
)

// RTC data are addressed by blocks of 8 bytes.
const blockSize = 8

type Block uint8

const (
	StatusStopped        = joybus.Status(0x80)
	StatusCrystalFailure = joybus.Status(0x02)
	StatusBatteryFailure = joybus.Status(0x01)
)

// RTC specific commands.
const (
	cmdInfo  = joybus.Command(0x06)
	cmdRead  = joybus.Command(0x07)
	cmdWrite = joybus.Command(0x08)
)

const (
	ID = joybus.DeviceID(0x0010)
)

type Rtc struct {
	joybus.DeviceImpl
}

func (d *Rtc) Info() (joybus.DeviceID, joybus.Status, error) {
	return joybus.Info(d.Joybus, cmdInfo)
}

// Low level command
// Doesn't take care of timing requirements nor storage unreliability.
func (d *Rtc) readBlock(block Block, data []byte) error {
	// TODO: test if accesses shorter than 8 bytes are supported
	if len(data) > blockSize {
		return ErrInvalidSize
	}

	if err := d.Execute(cmdRead, []byte{byte(block)}, data); err != nil {
		return err
	}

	return nil
}

// Low level command
// Doesn't take care of timing requirements nor storage unreliability.
func (d *Rtc) writeBlock(block Block, data []byte) (joybus.Status, error) {
	// TODO: test if accesses shorter than 8 bytes are supported
	if len(data) > blockSize {
		return 0, ErrInvalidSize
	}

	tx := append([]byte{byte(block)}, data...)
	rx := make([]byte, 1)

	if err := d.Execute(cmdWrite, tx, rx); err != nil {
		return 0, err
	}

	return joybus.Status(rx[0]), nil
}
