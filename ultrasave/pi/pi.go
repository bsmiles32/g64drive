package pi

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"sort"
)

var (
	ErrSizeProbeFailure = errors.New("unable to probe device size")
)

// Parallel Interface (PI) has a 32bit address space.
type Address uint32

// Interface that wrap the ReadWordAt method.
type WordReaderAt interface {
	// Perform a 32bit word IO read on PI bus
	// Note that address LSB is usually ignored by PI devices.
	ReadWordAt(address Address) (uint32, error)
}

// Interface that wrap the WriteWordAt method.
type WordWriterAt interface {
	// Perform a 32bit word IO write on PI bus
	// Note that address LSB is usually ignored by PI devices.
	WriteWordAt(word uint32, address Address) error
}

// Interface that wrap the ReadBurstAt method.
type BurstReaderAt interface {
	// Perform a burst read on PI bus
	// len(data) should be a multiple of 4.
	// Note that address LSB is usually ignored by PI devices.
	ReadBurstAt(data []byte, address Address) error
}

// Interface that wrap the WriteBurstAt method.
type BurstWriterAt interface {
	// Perform a burst write on PI bus
	// len(data) should be a multiple of 4.
	// Note that address LSB is usually ignored by PI devices.
	WriteBurstAt(data []byte, address Address) error
}

func strByteSize(size int) string {
	units := []string{"MiB", "KiB", "B"}

	multiplier := int(1024 * 1024)
	for _, u := range units {
		if size >= multiplier {
			if size%multiplier == 0 {
				return fmt.Sprintf("%d %s", size/multiplier, u)
			} else {
				return fmt.Sprintf("%.1f %s", float64(size)/float64(multiplier), u)
			}
		}

		multiplier /= 1024
	}

	return "0 B"
}

// Limitations:
// * doesn't work with word-addresses flash array
// * doesn't work with some repro carts's saves (or more precisely lack of save) as they don't necessarily report open bus values
func ProbeDeviceForKnownSizes(r WordReaderAt, baseAddress Address, knownSizes []int, openBusValidation, mirrorValidaton, maxRandOffset int, l Logger) (int, error) {
	// Ensure that size tests are done in increasing order
	// so that we report the smallest knownSize that exhibit mirroring / open bus.
	sort.Ints(knownSizes)
	for _, s := range knownSizes {

		// Reset validations counters
		nOpenBus := openBusValidation
		nMirror := mirrorValidaton

		log(l, "Probing for size: %s\n", strByteSize(s))
		for {
			offset := rand.Intn(maxRandOffset)

			// Check for open-bus behavior
			a := baseAddress + Address(s+offset)
			openBus := uint32(uint16(a))<<16 | uint32(uint16(a))
			log(l, "Probing @%08x=", a)
			w, err := r.ReadWordAt(a)
			if err != nil {
				log(l, "%s\n", err)
				return 0, err
			}
			log(l, "%08x", w)

			if w == openBus {
				log(l, ": open bus\n")
				if nOpenBus--; nOpenBus <= 0 {
					break
				}
				continue
			}

			// Check for mirroring behavior (only for a != a0 eg. size != 0)
			a0 := baseAddress + Address(offset)
			if a != a0 {
				log(l, " and @%08x=", a0)
				w0, err := r.ReadWordAt(a0)
				if err != nil {
					log(l, "%s\n", err)
					return 0, err
				}
				log(l, "%08x", w0)

				if w == w0 {
					log(l, ": mirror value\n")
					if nMirror--; nMirror <= 0 {
						break
					}
					continue
				}
			}

			log(l, ": normal value\n")
			break
		}

		if nOpenBus <= 0 || nMirror <= 0 {
			log(l, "size %s\n", strByteSize(s))
			return s, nil
		}
	}

	return 0, ErrSizeProbeFailure
}

// Helper function which will try to do a single burst write if supported,
// and fallback to many IO writes otherwise.
// This is helpful to workaround a bug in 64drive FW <2.04.
// len(data) must be a multiple of 4.
// Note that address LSB is usually ignored by PI devices.
func WriteBurstWithIOFallbackAt(w WordWriterAt, data []byte, address Address) (int, error) {
	// Check interfaces precondition
	if len(data)%4 != 0 {
		return 0, fmt.Errorf("burst size is not a multiple of 4 (%d)", len(data))
	}

	// We don't check address % 4 because it's not mandatory:
	// IO and Burst should behave the same.

	// If w support burst write use that
	if burstWriter, ok := w.(BurstWriterAt); ok {
		if err := burstWriter.WriteBurstAt(data, address); err != nil {
			return 0, err
		}
		return len(data), nil
	}

	// IO fallback
	for i := 0; i < len(data); i += 4 {
		u32 := binary.BigEndian.Uint32(data[i : i+4])
		if err := w.WriteWordAt(u32, address+Address(i)); err != nil {
			return i, err
		}
	}

	return len(data), nil
}

// Adapter
type BurstReaderAtFunc func([]byte, Address) error

func (b BurstReaderAtFunc) ReadBurstAt(data []byte, address Address) error {
	return b(data, address)
}

type BurstWriterAtFunc func([]byte, Address) error

func (b BurstWriterAtFunc) WriteBurstAt(data []byte, address Address) error {
	return b(data, address)
}

type BurstAtFunc func([]byte, Address) error

// Split transfer into bursts that don't cross page boundary.
// len(data) must be a multiple of 4.
// address must be a multiple of 4.
// pageBits must be greater or equal to 2.
func SplitBursts(ctx context.Context, data []byte, address Address, pageBits int, burstAt BurstAtFunc) (int, error) {
	// Check preconditions
	if len(data)%4 != 0 {
		return 0, fmt.Errorf("burst size is not a multiple of 4 (%d)", len(data))
	}

	if address%4 != 0 {
		return 0, fmt.Errorf("address is not a multiple of 4 (%08x)", address)
	}

	if pageBits < 2 {
		return 0, fmt.Errorf("pageBits must be greater or equal to 2 (%d)", pageBits)
	}

	begin := address
	end := address + Address(len(data))
	idx := 0

	for begin < end {
		if err := ctx.Err(); err != nil {
			return idx, err
		}

		burstEnd := alignUp(begin, pageBits)
		if burstEnd > end {
			burstEnd = end
		}

		burstSize := int(burstEnd - begin)

		if err := burstAt(data[idx:idx+burstSize], begin); err != nil {
			return idx, err
		}

		begin += Address(burstSize)
		idx += burstSize
	}

	return idx, nil
}

func alignDown(address Address, bits int) Address {
	mask := Address((1 << bits) - 1)
	return (address &^ mask)
}

func alignUp(address Address, bits int) Address {
	mask := Address((1 << bits) - 1)
	return (address | mask) + 1
}

// Splits a DMA Read operation into suitable bursts such that:
// * all burst have a size which is a multiple of 4 (to accommodate ultrasave constrains)
// * all burst are 4-byte aligned, this is a bit conservative as PI only need 2-byte alignment (for specified behavior) but this eases the implementation.
// * no burst will cross device page boundary (eg. 2^pageBits)
// * only the minimal number of burst shall be emitted (eg. we always try to read up to the next limit)
// * transparently handle unaligned transfers
func Read(ctx context.Context, p []byte, address Address, pageBits int, r BurstReaderAt) error {
	// Early return for empty reads
	if len(p) == 0 {
		return nil
	}

	pageSize := (1 << pageBits)
	var page = make([]byte, pageSize)

	// First burst realign transfer to next page, if reached, otherwise truncate to len(p).
	// Due to ultrasave limitation we also need to ensure end-begin is a multiple of 4
	// In case this require over-reading, we need to use a temporary buffer
	// and copy back only what's needed into p.
	begin := alignDown(address, 2)
	end := alignDown(address+Address(len(p))+3, 2)
	skip := int(address - begin)

	burstEnd := alignUp(begin, pageBits)
	if burstEnd > end {
		burstEnd = end
	}

	burstSize := int(burstEnd - begin)
	if err := r.ReadBurstAt(page[:burstSize], begin); err != nil {
		return err
	}

	idx := burstSize - skip
	if len(p) < idx {
		idx = len(p)
	}
	copy(p[:idx], page[skip:skip+idx])

	if idx == len(p) {
		return nil
	}

	// Next transfers should all be properly aligned without needing any over-reading
	begin = burstEnd
	for begin+Address(pageSize) < end {
		if err := ctx.Err(); err != nil {
			return err
		}

		// burstEnd < end by construction, no need to further limit burstEnd
		burstEnd := alignUp(begin, pageBits)
		burstSize := int(burstEnd - begin)
		if err := r.ReadBurstAt(p[idx:idx+burstSize], begin); err != nil {
			return err
		}

		begin += Address(burstSize)
		idx += burstSize
	}

	// Last transfer may need to over-read and copy back only required bytes.
	skip = int(end - (address + Address(len(p))))
	burstSize = int(end - begin)
	if err := r.ReadBurstAt(page[:burstSize], begin); err != nil {
		return err
	}
	copy(p[idx:], page[:burstSize-skip])

	return nil
}
