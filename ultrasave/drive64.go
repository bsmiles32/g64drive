package ultrasave

import (
	"fmt"
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
			joybusExecuter
		}{
			piWordReaderAt{d},
			piWordWriterAt{d},
			piBurstReaderAt{d},
			joybusExecuter{d},
		}
	}

	return struct {
		piWordReaderAt
		piWordWriterAt
		piBurstReaderAt
		piBurstWriterAt
		joybusExecuter
	}{
			piWordReaderAt{d},
			piWordWriterAt{d},
			piBurstReaderAt{d},
			piBurstWriterAt{d},
			joybusExecuter{d},
	}
}

// Implements pi.WordReaderAt
type piWordReaderAt struct { *drive64.Device }
func (d piWordReaderAt) ReadWordAt(address pi.Address) (uint32, error) {
	data, err := d.CmdStandAlonePiRead32(uint32(address))
	return data, convertError(err)
}

// Implements pi.WordWriterAt
type piWordWriterAt struct { *drive64.Device }
func (d piWordWriterAt) WriteWordAt(word uint32, address pi.Address) error {
	return convertError(d.CmdStandAlonePiWrite32(uint32(address), word))
}

// Implements pi.BurstReaderAt
type piBurstReaderAt struct { *drive64.Device }
func (d piBurstReaderAt) ReadBurstAt(data []byte, address pi.Address) error {
	return convertError(d.CmdStandAlonePiReadBurst(uint32(address), data))

}

// Implements pi.BurstWriterAt
type piBurstWriterAt struct { *drive64.Device }
func (d piBurstWriterAt) WriteBurstAt(data []byte, address pi.Address) error {
	return convertError(d.CmdStandAlonePiWriteBurst(uint32(address), data))
}

// Implements joybus.Executer interface
type joybusExecuter struct { *drive64.Device }
func (d joybusExecuter) Execute(cmd joybus.Command, tx, rx []byte) error {
	return convertError(d.CmdStandAloneSiOperation(append([]byte{byte(cmd)}, tx...), rx))
}

// To avoid leaking drive64 sentinel errors out of pi / joybus interfaces
// we create new errors from drive64 errors (no wrapping).
func convertError(err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%v", err)
}
