package rom

import (
	"context"
	"errors"
	"github.com/rasky/g64drive/ultrasave/pi"
)

var (
	ErrInvalidOffsetSize = errors.New("invalid offset / size")
)

// ROM are usually mapped at PI address 0x10000000.
const DefaultBaseAddress = pi.Address(0x10000000)

// This rom package requires these interfaces.
type Controller interface {
	pi.WordReaderAt
	pi.BurstReaderAt
}

type Rom struct {
	pi          Controller
	baseAddress pi.Address
	size        int
	logger      pi.Logger
}

func (r *Rom) BaseAddress() pi.Address {
	return r.baseAddress
}

func (r *Rom) Size() int {
	return r.size
}

type RomOption func(*Rom) error

// WithBaseAddress allows to override default base address.
func WithBaseAddress(baseAddress pi.Address) RomOption {
	return func(r *Rom) error {
		r.baseAddress = baseAddress
		return nil
	}
}

// WithSize allows to avoid auto detection of ROM size.
func WithSize(size int) RomOption {
	return func(r *Rom) error {
		r.size = size
		return nil
	}
}

func WithLogger(l pi.Logger) RomOption {
	return func(r *Rom) error {
		r.logger = l
		return nil
	}
}

func New(pi Controller, opts ...RomOption) (*Rom, error) {
	r := &Rom{
		pi:          pi,
		baseAddress: DefaultBaseAddress,
	}

	for _, opts := range opts {
		if err := opts(r); err != nil {
			return nil, err
		}
	}

	// Employ some heuristic to deduce ROM size
	if r.size == 0 {
		size, err := guessRomSize(pi, r.baseAddress, r.logger)
		if err != nil {
			return nil, err
		}
		r.size = size
	}

	return r, nil
}

func (r *Rom) Read(ctx context.Context, offset int, data []byte) error {
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

	return pi.Read(ctx, data, r.baseAddress+pi.Address(offset), 9, r.pi)
}

// Heuristic procedure to guess ROM size
func guessRomSize(r pi.WordReaderAt, baseAddress pi.Address, l pi.Logger) (int, error) {
	const MiB = 1024 * 1024

	// Do a first pass with official sizes
	// Assuming that we will deal primarily with official carts.
	knownSizes := []int{
		4 * MiB,
		8 * MiB,
		12 * MiB,
		16 * MiB,
		20 * MiB,
		24 * MiB,
		28 * MiB,
		32 * MiB,
		40 * MiB,
		64 * MiB,
	}

	s, err := pi.ProbeDeviceForKnownSizes(r, baseAddress, knownSizes, 5, 5, 32*1024, l)
	if err == nil {
		return s, nil
	}

	if err != pi.ErrSizeProbeFailure {
		return s, err
	}

	// TODO: try another approach for non standard cart ROM ? (homebrews ?)

	// For now assume Max ROM size
	return 64 * MiB, nil
}
