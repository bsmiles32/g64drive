package flash

import (
	"github.com/rasky/g64drive/drive64"
)

// Adapter between ParallelInterface and Drive64
// to allows interaction with a real flash in cart using 64drive ultrasave.
type Drive64ParallelInterfaceAdapter struct {
	*drive64.Device
}

func (d Drive64ParallelInterfaceAdapter) Read32(address Address) (uint32, error) {
	return d.CmdStandAlonePiRead32(uint32(address))
}

func (d Drive64ParallelInterfaceAdapter) Write32(address Address, data uint32) error {
	return d.CmdStandAlonePiWrite32(uint32(address), data)
}

func (d Drive64ParallelInterfaceAdapter) ReadBurst(address Address, data []byte) error {
	return d.CmdStandAlonePiReadBurst(uint32(address), data)
}

func (d Drive64ParallelInterfaceAdapter) WriteBurst(address Address, data []byte) error {
	err := d.CmdStandAlonePiWriteBurst(uint32(address), data)
	// convert drive64.ErrUnsupported to flash ErrUnsupported
	if err == drive64.ErrUnsupported {
		err = ErrUnsupported
	}
	return err
}
