package joybus

import (
	"encoding/binary"
	//"fmt"
	"github.com/rasky/g64drive/ultrasave"
)

type Command uint8

// Joybus common commands.
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
	SI ultrasave.SerialInterface
}

func (d *DeviceImpl) Info() (DeviceID, Status, error) {
	return Info(d.SI, cmdInfo)
}

func (d *DeviceImpl) Reset() (DeviceID, Status, error) {
	return Info(d.SI, cmdReset)
}

func Info(si ultrasave.SerialInterface, cmd Command) (DeviceID, Status, error) {
	var info [3]byte

	if err := Operation(si, cmd, nil, info[:]); err != nil {
		return 0, 0, err
	}

	id := DeviceID(binary.BigEndian.Uint16(info[:2]))
	status := Status(info[2])

	return id, status, nil
}

func Operation(si ultrasave.SerialInterface, cmd Command, tx, rx []byte) error {
	err := si.Operation(append([]byte{byte(cmd)}, tx...), rx)
	if err != nil {
		//fmt.Printf("SI operation: cmd=%02x tx=%v error=%v\n", cmd, tx, err)
		return err
	}

	//fmt.Printf("SI operation: cmd=%02x tx=%v rx=%v\n", cmd, tx, rx)
	return nil
}
