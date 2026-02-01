package eeprom

// TODO: verify accepted commands
// TODO: give higher level functions for IO

import (
	"errors"

	"github.com/rasky/g64drive/ultrasave/joybus"
)

var (
	ErrInvalidSize = errors.New("invalid block access size")
)

func init() {
	joybus.RegisterDevices(joybus.RegisteredDevices{
		Probe: joybus.RegularProbe,
		Factories: joybus.Factories{
			ID4Kib: joybus.Factory{
				Name:    "EEPROM 4Kib",
				Factory: factory,
			},
			ID16Kib: joybus.Factory{
				Name:    "EEPROM 16Kib",
				Factory: factory,
			},
		},
	})
}

// EEPROM are addressed by blocks of 8 bytes.
const blockSize = 8

type Block uint8

const (
	StatusBusy = joybus.Status(0x80)
)

// EEPROM specific commands.
const (
	cmdRead  = joybus.Command(0x04)
	cmdWrite = joybus.Command(0x05)
)

// There are 2 known EEPROM device ID
const (
	// 4kbits = 512 bytes
	ID4Kib = joybus.DeviceID(0x0080)
	// 16kbits = 2048 bytes
	ID16Kib = joybus.DeviceID(0x00c0)
)

type Eeprom struct {
	joybus.DeviceImpl
}

func New(c joybus.Controller) *Eeprom {
	return &Eeprom{joybus.DeviceImpl{c}}
}

func factory(_ joybus.DeviceID, c joybus.Controller) joybus.Device { return New(c) }

// Low level command
// Doesn't take care of timing requirements nor storage unreliability.
// TOVERIFY: does it returns a status byte at the end ?
func (d *Eeprom) ReadBlock(block Block, data []byte) error {
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
func (d *Eeprom) WriteBlock(block Block, data []byte) (joybus.Status, error) {
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
