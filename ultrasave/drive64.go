package ultrasave

import (
	"errors"
	"time"

	"github.com/rasky/g64drive/drive64"
	"github.com/rasky/g64drive/ultrasave/joybus"
	"github.com/rasky/g64drive/ultrasave/pi"
)

func New64DriveAdapters(d *drive64.Device) interface{} {
	// PiWriteBurst doesn't work on firmware <2.04
	_, fwver, _, err := d.CmdVersionRequest()
	if err != nil {
		return err
	}
	if fwver < 204 {
		return struct {
			piWordReaderAt
			piWordWriterAt
			piBurstReaderAt
			joybusController
		}{
			piWordReaderAt{d},
			piWordWriterAt{d},
			piBurstReaderAt{d},
			joybusController{d},
		}
	}

	return struct {
		piWordReaderAt
		piWordWriterAt
		piBurstReaderAt
		piBurstWriterAt
		joybusController
	}{
		piWordReaderAt{d},
		piWordWriterAt{d},
		piBurstReaderAt{d},
		piBurstWriterAt{d},
		joybusController{d},
	}
}

// Implements pi.WordReaderAt
type piWordReaderAt struct{ *drive64.Device }

func (d piWordReaderAt) ReadWordAt(address pi.Address) (uint32, error) {
	return d.CmdStandAlonePiRead32(uint32(address))
}

// Implements pi.WordWriterAt
type piWordWriterAt struct{ *drive64.Device }

func (d piWordWriterAt) WriteWordAt(word uint32, address pi.Address) error {
	return d.CmdStandAlonePiWrite32(uint32(address), word)
}

// Implements pi.BurstReaderAt
type piBurstReaderAt struct{ *drive64.Device }

func (d piBurstReaderAt) ReadBurstAt(data []byte, address pi.Address) error {
	return d.CmdStandAlonePiReadBurst(uint32(address), data)

}

// Implements pi.BurstWriterAt
type piBurstWriterAt struct{ *drive64.Device }

func (d piBurstWriterAt) WriteBurstAt(data []byte, address pi.Address) error {
	return d.CmdStandAlonePiWriteBurst(uint32(address), data)
}

// Implements joybus.Controller interface
type joybusController struct{ *drive64.Device }

func (d joybusController) Execute(cmd joybus.Command, tx, rx []byte) error {
	return d.CmdStandAloneSiOperation(append([]byte{byte(cmd)}, tx...), rx)
}

func (d joybusController) Unfreeze(err error) bool {
	if errors.Is(err, drive64.ErrFrozen) {
		// XXX: This "magic" procedure allows to "unfreeze" 64drive / SI device
		// so SI device can accept further commands (after having received an unknown command).
		// I don't have a good understanding of why this work and why this is needed,
		// but it seems to work on 64drive HW1 FW2.03 and 64drive HW2 FW2.05 (linux).
		time.Sleep(2700 * time.Millisecond)
		d.Reset()
		return true
	}

	return false
}
