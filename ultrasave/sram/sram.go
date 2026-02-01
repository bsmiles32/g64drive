package sram

import (
	"context"
	"errors"

	"github.com/rasky/g64drive/ultrasave/logger"
	"github.com/rasky/g64drive/ultrasave/pi"
)

func init() {
	pi.RegisterDeviceClass(pi.DeviceClass{
		Priority: 2,
		Name:     "SRAM",
		Probe:    probe,
		Factory:  factory,
	})
}

var (
	ErrControllerRequirementsNotMet = errors.New("controller requirements not met")
	ErrInvalidOffsetSize            = errors.New("invalid offset / size")
)

// SRAM are usually mapped at PI address 0x08000000.
const DefaultBaseAddress = pi.Address(0x08000000)

// This sram package requires these interfaces.
type Controller interface {
	pi.WordReaderAt
	pi.WordWriterAt
	pi.BurstReaderAt
}

type SRAM struct {
	pi          Controller
	baseAddress pi.Address
	size        int
}

func (r *SRAM) BaseAddress() pi.Address {
	return r.baseAddress
}

func (r *SRAM) Size() int {
	return r.size
}

type SRAMOption func(*SRAM) error

// WithBaseAddress allows to override default base address.
func WithBaseAddress(baseAddress pi.Address) SRAMOption {
	return func(r *SRAM) error {
		r.baseAddress = baseAddress
		return nil
	}
}

func New(pi Controller, size int, opts ...SRAMOption) (*SRAM, error) {
	r := &SRAM{
		pi:          pi,
		baseAddress: DefaultBaseAddress,
		size:        size,
	}

	for _, opts := range opts {
		if err := opts(r); err != nil {
			return nil, err
		}
	}

	return r, nil
}

func (r *SRAM) Read(ctx context.Context, offset int, data []byte) error {
	// Validate offset size
	if offset < 0 || offset >= r.size {
		return ErrInvalidOffsetSize
	}
	if len(data) > r.size {
		return ErrInvalidOffsetSize
	}
	if offset+len(data) > r.size {
		return ErrInvalidOffsetSize
	}

	return pi.Read(ctx, data, r.baseAddress+pi.Address(offset), 15, r.pi)
}

func probe(c pi.ProbeController, baseAddress pi.Address, l logger.Logger) (bool, int, error) {
	// Skip if SRAM not at defaultBaseAddress + (k << 18) (k = 0..3)
	if baseAddress&0xfff3ffff != DefaultBaseAddress {
		return false, 0, nil
	}

	logger.Log(l, "Probing for SRAM at %08x\n", baseAddress)

	// Assume that a device is present (eg. open-bus test at baseAddress is already negative)

	// Check for known memory size at baseAddress.
	// Use a large number of mirror validation because in a lot of save content
	// data is replicated at many addresses which would cause false positive result
	// for the mirroring test.
	knownSizes := []int{
		32 * 1024, // SRAM (256Kib / 32KiB)
	}

	size, err := pi.ProbeDeviceForKnownSizes(c, baseAddress, knownSizes, 5, 4*1024, 32*1024, l)
	if err != nil && !errors.Is(err, pi.ErrSizeProbeFailure) {
		return false, 0, err
	}
	if err == nil {
		return true, size, nil
	}

	// Non conclusive
	return false, 0, nil
}

func factory(c interface{}, baseAddress pi.Address, size int) (interface{}, error) {
	pi, ok := c.(Controller)
	if !ok {
		return nil, ErrControllerRequirementsNotMet
	}

	return New(pi, size, WithBaseAddress(baseAddress))
}
