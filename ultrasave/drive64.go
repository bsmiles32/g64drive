package ultrasave

import (
	"github.com/rasky/g64drive/drive64"
	"github.com/rasky/g64drive/ultrasave/joybus"
	"github.com/rasky/g64drive/ultrasave/pi"
)

// Adapter between pi.ParallelInterface and Drive64
// to allows interaction with a real PI device in cart using 64drive ultrasave.
type Drive64PIAdapter struct {
	*drive64.Device
}

func (d Drive64PIAdapter) Read32(address pi.Address) (uint32, error) {
	return d.CmdStandAlonePiRead32(uint32(address))
}

func (d Drive64PIAdapter) Write32(address pi.Address, data uint32) error {
	return d.CmdStandAlonePiWrite32(uint32(address), data)
}

func (d Drive64PIAdapter) ReadBurst(address pi.Address, data []byte) error {
	return d.CmdStandAlonePiReadBurst(uint32(address), data)
}

func (d Drive64PIAdapter) WriteBurst(address pi.Address, data []byte) error {
	err := d.CmdStandAlonePiWriteBurst(uint32(address), data)
	// convert drive64.ErrUnsupported to pi ErrUnsupported
	if err == drive64.ErrUnsupported {
		err = pi.ErrUnsupported
	}
	return err
}

// Adapter between si.SerialInterface and Drive64
// to allows interaction with a real SI device in cart using 64drive ultrasave.
type Drive64JoybusAdapter struct {
	*drive64.Device
}

func (d Drive64JoybusAdapter) Execute(cmd joybus.Command, tx, rx []byte) error {
	return d.CmdStandAloneSiOperation(append([]byte{byte(cmd)}, tx...), rx)
}
