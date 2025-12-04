package eeprom

import (
	"errors"
	"github.com/rasky/g64drive/ultrasave/joybus"
)

var (
	ErrInvalidSize = errors.New("invalid block access size")
)

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
	ID4kb = joybus.DeviceID(0x0080)
	// 16kbits = 2048 bytes
	ID16kb = joybus.DeviceID(0x00c0)
)

type Eeprom struct {
	joybus.DeviceImpl
}

// Low level command
// Doesn't take care of timing requirements nor storage unreliability.
func (d *Eeprom) ReadBlock(block Block, data []byte) error {
	// TODO: test if accesses shorter than 8 bytes are supported
	if len(data) > blockSize {
		return ErrInvalidSize
	}

	if err := joybus.Operation(d.SI, cmdRead, []byte{byte(block)}, data); err != nil {
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

	if err := joybus.Operation(d.SI, cmdWrite, tx, rx); err != nil {
		return 0, err
	}

	return joybus.Status(rx[0]), nil
}
