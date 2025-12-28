package pi

import (
	"context"
	"errors"
)

var (
	// Implementors of ParallelInterface may return
	// this error if the requested operation is not supported.
	ErrUnsupported = errors.New("operation is not supported")
	ErrUnalignedBurst = errors.New("unaligned burst")
)

// Parallel Interface (PI) has a 32bit address space.
type Address uint32

// Abstract PI Controller.
type Controller interface {
	Read32(address Address) (uint32, error)
	Write32(address Address, data uint32) error
	ReadBurst(address Address, data []byte) error
	WriteBurst(address Address, data []byte) error
}

func alignDown(address Address, bits int) Address {
	mask := Address((1 << bits) - 1)
	return (address &^ mask)
}

func alignUp(address Address, bits int) Address {
	mask := Address((1 << bits) - 1)
	return (address | mask) + 1
}

type BurstFn func(Address, []byte) error

// Splits a DMA Read operation into suitable bursts such that:
// * all burst have a size which is a multiple of 4 (to accomodate ultrasave constrains)
// * all burst are 4-byte aligned, this is a bit conservative as PI only need 2-byte alignment (for specified behavior) but this eases the implementation.
// * no burst will cross device page boundary (eg. 2^pageBits)
// * only the minimal number of burst shall be emitted (eg. we always try to read up to the next limit)
// * transparently handle unaligned transfers
func Read(ctx context.Context, p []byte, address Address, pageBits int, readBurst BurstFn) error {
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
	if err := readBurst(begin, page[:burstSize]); err != nil {
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
		if err := readBurst(begin, p[idx:idx+burstSize]); err != nil {
			return err
		}

		begin += Address(burstSize)
		idx += burstSize
	}

	// Last transfer may need to over-read and copy back only required bytes.
	skip = int(end - (address + Address(len(p))))
	burstSize = int(end - begin)
	if err := readBurst(begin, page[:burstSize]); err != nil {
		return err
	}
	copy(p[idx:], page[:burstSize-skip])

	return nil
}
