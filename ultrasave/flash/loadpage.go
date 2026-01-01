package flash

import (
	//"encoding/binary"
	"github.com/rasky/g64drive/ultrasave/pi"
)

// Load a maximum of 128 bytes into internal page before programming.
// Assume erased state of internal page (eg. all bits set).
func (f *Flash) LoadBytePage(data []byte) error {
	// 64drive only support bursts length which are multiple of 4.
	if len(data) > f.layout.PageSize() || len(data)%4 != 0 {
		return ErrInvalidPageSize
	}

	if err := f.writeCIR(cmdLoadBytePage); err != nil {
		return err
	}

	// Use WriteAt which will fallback to IO code path if burst is not supported
	// (workaround bug in 64drive FW < 2.04)
	_, err := pi.WriteAt(f.pi, data, f.baseAddress)
	return err
}
