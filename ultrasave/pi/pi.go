package pi

import (
	"context"
	"encoding/binary"
)

// Parallel Interface (PI) has a 32bit address space.
type Address uint32

// Interface that wrap the ReadWordAt method.
type WordReaderAt interface {
	// Perform a 32bit word IO read on PI bus
	// address should be a multiple of 4.
	ReadWordAt(address Address) (uint32, error)
}

// Interface that wrap the WriteWordAt method.
type WordWriterAt interface {
	// Perform a 32bit word IO write on PI bus
	// address should be a multiple of 4.
	WriteWordAt(word uint32, address Address) error
}

// Interface that wrap the ReadBurstAt method.
type BurstReaderAt interface {
	// Perform a burst read on PI bus
	// Both address and len(data) should be a multiple of 4.
	ReadBurstAt(data []byte, address Address) error
}

// Interface that wrap the WriteBurstAt method.
type BurstWriterAt interface {
	// Perform a burst write on PI bus
	// Both address and len(data) should be a multiple of 4.
	WriteBurstAt(data []byte, address Address) error
}

// Helper function which will try to do a burst write if supported,
// and fallback to IO write otherwise.
// This is helpful to workaround a bug in 64drive FW <2.04.
// Both address and len(data) should be a multiple of 4.
func WriteAt(w WordWriterAt, data []byte, address Address) (int, error) {
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
		if err := w.WriteWordAt(u32, address + Address(i)); err != nil {
			return i, err
		}
	}

	return len(data), nil
}







func alignDown(address Address, bits int) Address {
	mask := Address((1 << bits) - 1)
	return (address &^ mask)
}

func alignUp(address Address, bits int) Address {
	mask := Address((1 << bits) - 1)
	return (address | mask) + 1
}

type BurstFn func([]byte, Address) error

// Splits a DMA Read operation into suitable bursts such that:
// * all burst have a size which is a multiple of 4 (to accommodate ultrasave constrains)
// * all burst are 4-byte aligned, this is a bit conservative as PI only need 2-byte alignment (for specified behavior) but this eases the implementation.
// * no burst will cross device page boundary (eg. 2^pageBits)
// * only the minimal number of burst shall be emitted (eg. we always try to read up to the next limit)
// * transparently handle unaligned transfers
func Read(ctx context.Context, p []byte, address Address, pageBits int, readBurstAt BurstFn) error {
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
	if err := readBurstAt(page[:burstSize], begin); err != nil {
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
		if err := readBurstAt(p[idx:idx+burstSize], begin); err != nil {
			return err
		}

		begin += Address(burstSize)
		idx += burstSize
	}

	// Last transfer may need to over-read and copy back only required bytes.
	skip = int(end - (address + Address(len(p))))
	burstSize = int(end - begin)
	if err := readBurstAt(page[:burstSize], begin); err != nil {
		return err
	}
	copy(p[idx:], page[:burstSize-skip])

	return nil
}

/*
type PagedDevice struct {
	ctx context.Context
	pageBits int
	readBurstAt BurstFn
	writeBurst BurstFn
}

func (r *PageReader) ReadAt(ctx context.Context, p []byte, address int64) (nread int, err error) {
}
*/
