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

func New(pi Controller, size int, opts ...RomOption) (*Rom, error) {
	r := &Rom{
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
