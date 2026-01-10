package rom

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
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

// WithSize allows to avoid auto detection of ROM size.
func WithSize(size int) RomOption {
	return func(r *Rom) error {
		r.size = size
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
		size, err := guessRomSize(pi, r.baseAddress)
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
// TODO: refactor into a function in pi suitable for both cart ROM and SRAM (and Flash Array ?)
func guessRomSize(r pi.WordReaderAt, baseAddress pi.Address) (int, error) {
	const MiB = 1024 * 1024

	// Do a first pass with official sizes
	// Assuming that we will deal primarily with official carts.
	knownSizes := []int{
		4*MiB,
		8*MiB,
		12*MiB,
		16*MiB,
		20*MiB,
		24*MiB,
		28*MiB,
		32*MiB,
		40*MiB,
		64*MiB,
	}

	for _, s := range knownSizes {
		// Check at multiple (N) random offset close but beyond s that we either get open bus or mirroring
		// We assume ascending order of size tests for the w == w0 test.
		fmt.Printf("Probing ROM for size = %d MiB\n", s / MiB)
		k := 0
		const N = 5
		for ; k < N; k++ {
			offset := rand.Intn(4096)

			openBus := uint32(offset) << 16 | uint32(offset)

			a0 := baseAddress + pi.Address(offset)
			fmt.Printf("Probing ROM @%08x", a0)
			w0, err := r.ReadWordAt(a0)
			if err != nil {
				fmt.Println()
				return 0, err
			}
			fmt.Printf(":%08x\n", w0)

			a := baseAddress + pi.Address(s + offset)
			fmt.Printf("Probing ROM @%08x", a)
			w, err := r.ReadWordAt(a)
			if err != nil {
				fmt.Println()
				return 0, err
			}
			fmt.Printf(":%08x\n", w)

			// Verify that any at offset beyond s returns either openBus or w0
			if w == openBus {
				fmt.Println("Got open-bus value")
				continue
			} else if w == w0 {
				fmt.Println("Got mirroring value")
				continue
			}

			fmt.Println("End of ROM not reached yet")
			break
		}

		if k == N {
			fmt.Printf("ROM size %d MiB\n", s / MiB)
			return s, nil
		}
	}

	// TODO: second pass for homebrews ROMs that don't fit with known sizes ?

	// Cold not deduce size of ROM assume max 64MiB
	return 64 * MiB, nil
}
