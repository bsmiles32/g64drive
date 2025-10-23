// Package flash implements low level routines to interact with
// flash chips found in N64 cartridge.
// It also provides higher level IO functions for convenience.
package flash

// Flash is accessed through Parallel Interface using 32bit addresses.
type Address uint32

// Flash are usually mapped at PI address 0x08000000.
const DefaultBaseAddress = Address(0x08000000)

// ParallelInterface provides 32bit IO and burst IO.
// This interface decouple and abstract all interactions with the PI.
type ParallelInterface interface {
	Read32(address Address) (uint32, error)
	Write32(address Address, data uint32) error
	ReadBurst(address Address, data []byte) error
	WriteBurst(address Address, data []byte) error
}

type Flash struct {
	pi          ParallelInterface
	baseAddress Address
	layout      Layout
}

func New(pi ParallelInterface, opts ...FlashOption) (*Flash, error) {
	f := &Flash{
		pi:          pi,
		baseAddress: DefaultBaseAddress,
	}

	for _, opts := range opts {
		if err := opts(f); err != nil {
			return nil, err
		}
	}

	// Deduce layout based on SiliconID if no layout were specified
	if f.layout == (Layout{}) {
		siliconID, err := f.SiliconID()
		if err != nil {
			return nil, err
		}
		f.layout = layoutFromSiliconID(siliconID)

		// put back device in ReadArray mode to mimic initial state
		if err := f.writeCIR(cmdReadArray); err != nil {
			return nil, err
		}
	}

	return f, nil
}

// Get flash layout informations
func (f *Flash) Layout() Layout {
	return f.layout
}

// Deduce layout based on SiliconID.
// TOVERIFY: is the layout really based on SiliconID or it's just that
// word based chip are configured like that
// because they have a ^BYTE pin = 1 on cart PCB as suggested by MX29L1611 datasheet.
func layoutFromSiliconID(sID *SiliconID) Layout {
	switch sID.U32()[1] {
	case 0x00c20000, 0x00c20001, 0x00c2001e:
		return Layout_64W_128_8
	default:
		return Layout_128B_128_8
	}
}
