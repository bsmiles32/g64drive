package flash

import (
	"encoding/binary"
	"errors"
	"math/rand"

	"github.com/rasky/g64drive/ultrasave/logger"
	"github.com/rasky/g64drive/ultrasave/pi"
)

func probe(c pi.ProbeController, baseAddress pi.Address, l logger.Logger) (bool, int, error) {
	// Skip if flash not at expected base address
	// FIXME?: theoretically there could be multiple flash at PI addresses
	// 0x08000000 + k * 0x20000 but never seen in practice.
	// eg. we could relax a bit this inequality.
	if baseAddress != DefaultBaseAddress {
		return false, 0, nil
	}

	logger.Log(l, "Probing for flash at %08x\n", baseAddress)

	// Assume that a device is present (eg. open-bus test at baseAddress is already negative)

	// Check if a word-indexed flash array is mapped at baseAddress
	// by looking at a mismatch between IO and DMA:
	// A single DMA at baseAddress (smaller than PI page size) will fetch bytes
	// that we can compare against IO at corresponding addresses.
	// For word-indexed flash array, IO read at offset should match DMA at offset/2
	// and may differ from DMA at offset.
	// We do this test early because pi.ProbeDeviceForKnownSizes mirror test doesn't work
	// with word-indexed memory.
	// Limitation: this test doesn't work if the first 256*128 bytes of flash Data Array
	// are all the same.
	logger.Log(l, "Probing for word-indexed flash array\n")
	data := make([]byte, 256*128)
	if err := c.ReadBurstAt(data, baseAddress); err != nil {
		return false, 0, err
	}
	for k := 0; k < len(data)/4; k++ {
		// align reads to 4 bytes, but skip offset 0 because
		// burst and IO will always be equal for byte and word indexed flash.
		offset := 4 + uint32(rand.Intn(len(data)-4)) & ^uint32(3)

		w, err := c.ReadWordAt(baseAddress + pi.Address(offset))
		if err != nil {
			return false, 0, err
		}
		u32_b := binary.BigEndian.Uint32(data[offset : offset+4])
		u32_w := binary.BigEndian.Uint32(data[offset/2 : offset/2+4])
		if u32_b != w && u32_w == w {
			logger.Log(l, "Mismatch between IO and burst (offset=%08x, %08x, %08x). Assuming flash save type\n", offset, w, u32_b)
			return true, 128 * 1024, nil
		}
	}

	// Check if flash SiliconID is mapped at baseAddress:
	// In SiliconID mode, offset is (mostly) ignored, only burst size matter
	// and first u32 should match flash.ExpectedTypeID.
	logger.Log(l, "Probing for flash silicon id\n")
	data = make([]byte, 8)
	isFlash := true
	for k := 0; k < 200; k++ {
		offset := rand.Intn(0x4000 - 8)
		if err := c.ReadBurstAt(data, baseAddress+pi.Address(offset)); err != nil {
			return false, 0, err
		}

		u32 := binary.BigEndian.Uint32(data[0:4])
		if u32 != ExpectedTypeID {
			isFlash = false
			break
		}
	}
	if isFlash {
		logger.Log(l, "Found flash silicon ID (%02x). Assuming flash save type\n", data)
		return true, 128 * 1024, nil
	}

	// TODO?: Handle flash in status and loadpage mode
	// This is low priority because on poweron flash should be in ReadArray mode,
	// so these mode can only happen if some previous operations have been done prior probing.

	// Check for known memory size at baseAddress.
	// Use a large number of mirror validation because in a lot of save content
	// data is replicated at many addresses which would cause false positive result
	// for the mirroring test.
	knownSizes := []int{
		32 * 1024,  // If mirror behavior here -> SRAM
		128 * 1024, // 1Mib => Assume Flash based on size
	}
	size, err := pi.ProbeDeviceForKnownSizes(c, baseAddress, knownSizes, 5, 4*1024, 32*1024, l)
	if err != nil && !errors.Is(err, pi.ErrSizeProbeFailure) {
		return false, 0, err
	}
	if err == nil && size == 128*1024 {
		return true, size, nil
	}

	// Non conclusive
	return false, 0, nil
}
