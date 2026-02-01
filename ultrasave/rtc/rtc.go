package rtc

// TODO: verify accepted commands
// TODO: give higher level functions to set time / alarm, access SRAM, ...

import (
	"errors"

	"github.com/rasky/g64drive/ultrasave/joybus"
)

var (
	ErrInvalidSize = errors.New("invalid block access size")
	ErrInvalidBlock = errors.New("invalid block number")
)

func init() {
	joybus.RegisterDevices(joybus.RegisteredDevices{
		Probe: joybus.OrderedProbingCommand{
			Priority: 0,
			Name: "rtc",
			Command: cmdInfo,
		},
		Factories: joybus.Factories{
			ID: joybus.Factory{
				Name:    "RTC",
				Factory: factory,
			},
		},
	})
}

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
	// FIXME?: Not sure if RTC supports cmdReset command, so might not be wise to derive from DeviceImpl
	joybus.DeviceImpl
}

func New(c joybus.Controller) *Rtc {
	return &Rtc{ joybus.DeviceImpl{ c } }
}

func factory(_ joybus.DeviceID, c joybus.Controller) joybus.Device { return New(c) }

// Override DeviceImpl.Info so that it uses proper rtc.cmdInfo
func (d *Rtc) Info() (joybus.DeviceID, joybus.Status, error) {
	return joybus.Info(d, cmdInfo)
}

// Low level command
// Doesn't take care of timing requirements nor storage unreliability.
func (d *Rtc) readBlock(block Block, data []byte) (joybus.Status, error) {
	// TODO: test if accesses shorter than 8 bytes are supported
	if len(data) > blockSize {
		return 0, ErrInvalidSize
	}
	if block > 3 {
		return 0, ErrInvalidBlock
	}

	// Return block data and status byte
	buf := make([]byte, blockSize + 1)
	if err := d.Execute(cmdRead, []byte{byte(block)}, buf); err != nil {
		return 0, err
	}
	copy(data, buf[:blockSize])
	return joybus.Status(buf[blockSize]), nil
}

// Low level command
// Doesn't take care of timing requirements nor storage unreliability.
func (d *Rtc) writeBlock(block Block, data []byte) (joybus.Status, error) {
	// TODO: test if accesses shorter than 8 bytes are supported
	if len(data) > blockSize {
		return 0, ErrInvalidSize
	}
	if block > 3 {
		return 0, ErrInvalidBlock
	}

	tx := append([]byte{byte(block)}, data...)
	rx := make([]byte, 1)

	if err := d.Execute(cmdWrite, tx, rx); err != nil {
		return 0, err
	}

	return joybus.Status(rx[0]), nil
}
