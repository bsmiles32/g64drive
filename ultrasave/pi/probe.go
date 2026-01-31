package pi

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"sort"

	"github.com/rasky/g64drive/ultrasave/logger"
)

var (
	ErrSizeProbeFailure = errors.New("unable to probe device size")
)

type DeviceType int

const (
	None DeviceType = iota
	FlashRAM
	ROM
	SRAM
)

type ProbedDevice struct {
	Device      DeviceType
	BaseAddress Address
	Size        int
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
// * doesn't work when data is indistinguishable from mirroring
func probeDeviceForKnownSizes(r WordReaderAt, baseAddress Address, knownSizes []int, openBusValidation, mirrorValidaton, maxRandOffset int, l logger.Logger) (int, error) {
	// Ensure that size tests are done in increasing order
	// so that we report the smallest knownSize that exhibit mirroring / open bus.
	sort.Ints(knownSizes)
	for _, s := range knownSizes {

		// Reset validations counters
		nOpenBus := openBusValidation
		nMirror := mirrorValidaton

		logger.Log(l, "Probing for size: %s\n", strByteSize(s))
		for {
			offset := rand.Intn(maxRandOffset)

			// Check for open-bus behavior
			a := baseAddress + Address(s+offset)
			openBus := uint32(uint16(a))<<16 | uint32(uint16(a))
			logger.Log(l, "Probing @%08x=", a)
			w, err := r.ReadWordAt(a)
			if err != nil {
				logger.Log(l, "%s\n", err)
				return 0, err
			}
			logger.Log(l, "%08x", w)

			if w == openBus {
				logger.Log(l, ": open bus\n")
				if nOpenBus--; nOpenBus <= 0 {
					break
				}
				continue
			}

			// Check for mirroring behavior (only for a != a0 eg. size != 0)
			a0 := baseAddress + Address(offset)
			if a != a0 {
				logger.Log(l, " and @%08x=", a0)
				w0, err := r.ReadWordAt(a0)
				if err != nil {
					logger.Log(l, "%s\n", err)
					return 0, err
				}
				logger.Log(l, "%08x", w0)

				if w == w0 {
					logger.Log(l, ": mirror value\n")
					if nMirror--; nMirror <= 0 {
						break
					}
					continue
				}
			}

			logger.Log(l, ": normal value\n")
			break
		}

		if nOpenBus <= 0 || nMirror <= 0 {
			return s, nil
		}
	}

	return 0, ErrSizeProbeFailure
}

// Heuristic to detect (presence and) size of cart ROM
func ProbeCartRom(r WordReaderAt, l logger.Logger) (ProbedDevice, error) {
	const MiB = 1024 * 1024

	baseAddress := Address(0x10000000)

	// Do a first pass with official sizes
	// Assuming that we will deal primarily with official carts,
	// And that a ROM is always present.
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

	size, err := probeDeviceForKnownSizes(r, baseAddress, knownSizes, 5, 5, 32*1024, l)
	if errors.Is(err, ErrSizeProbeFailure) {
		// TODO: try another approach for non standard cart ROM ? (homebrews ?)
		// For now assume a Max ROM size of 64MiB
		size = 64 * MiB
		err = nil
		logger.Log(l, "Unable to guess ROM size, assuming %d MiB", size/MiB)
	} else if err != nil {
		return ProbedDevice{}, err
	} else {
		logger.Log(l, "Guessing a ROM size of %d MiB\n", size/MiB)
	}

	return ProbedDevice{
		Device:      ROM,
		BaseAddress: baseAddress,
		Size:        size,
	}, nil
}

/*
Objective of this procedure is to detect the kind of save (None, SRAM, FlashRAM)
and it's size.
To prevent any data loss we actively refrain from using write operations at this stage.
Note that data mapped by FlashRAM at base address depends on it's current mode. It can
be any of the following buffer: Data Array (=default mode on poweron), Status, SiliconID,
Internal Page Buffer. Also depending on FlashRAM variant Data Array buffer may be byte
or word indexed, which complexify detection (as we can't rely on writes).
*/
func ProbeCartSave(r interface {
	WordReaderAt
	BurstReaderAt
}, l logger.Logger) (ProbedDevice, error) {
	baseAddress := Address(0x08000000)

	// Check if some device is mapped at baseAddress
	logger.Log(l, "Probing for open-bus values at baseAddress\n")
	size, err := probeDeviceForKnownSizes(r, baseAddress, []int{0}, 5, 5, 32*1024, l)
	if err != nil && !errors.Is(err, ErrSizeProbeFailure) {
		return ProbedDevice{}, err
	}
	if size == 0 && !errors.Is(err, ErrSizeProbeFailure) {
		return ProbedDevice{
			Device:      None,
			BaseAddress: baseAddress,
			Size:        0,
		}, nil
	}

	// Check if a word-indexed flash array is mapped at baseAddress
	// by looking at a mismatch between IO and DMA:
	// A single DMA at baseAddress (smaller than PI page size) will fetch bytes
	// that we can compare against IO at corresponding addresses.
	// For word-indexed flash array, IO read at offset should match DMA at offset/2
	// and may differ from DMA at offset.
	// We do this test early because ProbeDeviceForKnownSizes mirror test doesn't work
	// with word-indexed memory.
	// Limitation: this test doesn't work if the first 256*128 bytes of flash Data Array
	// are all the same.
	logger.Log(l, "Probing for word-indexed flash array\n")
	data := make([]byte, 256*128)
	if err := r.ReadBurstAt(data, baseAddress); err != nil {
		return ProbedDevice{}, err
	}
	for k := 0; k < (256*128)/4; k++ {
		// align reads to 4 bytes, but skip offset 0 because
		// burst and IO will always be equal for byte and word indexed flash.
		offset := 4 + uint32(rand.Intn(len(data)-4)) & ^uint32(3)

		w, err := r.ReadWordAt(baseAddress + Address(offset))
		if err != nil {
			return ProbedDevice{}, err
		}
		u32_b := binary.BigEndian.Uint32(data[offset : offset+4])
		u32_w := binary.BigEndian.Uint32(data[offset/2 : offset/2+4])
		if u32_b != w && u32_w == w {
			logger.Log(l, "Mismatch between IO and burst (offset=%08x, %08x, %08x). Assuming flash save type\n", offset, w, u32_b)
			return ProbedDevice{
				Device:      FlashRAM,
				BaseAddress: baseAddress,
				Size:        128 * 1024,
			}, nil
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
		if err := r.ReadBurstAt(data, baseAddress+Address(offset)); err != nil {
			return ProbedDevice{}, err
		}

		u32 := binary.BigEndian.Uint32(data[0:4])
		if u32 != uint32(0x11118001) {
			isFlash = false
			break
		}
	}
	if isFlash {
		logger.Log(l, "Found flash silicon ID (%02x). Assuming flash save type\n", data)
		return ProbedDevice{
			Device:      FlashRAM,
			BaseAddress: baseAddress,
			Size:        128 * 1024,
		}, nil
	}

	// TODO?: Handle flash in status and loadpage mode
	// This is low priority because on poweron flash should be in ReadArray mode,
	// so these mode can only happen if some previous operations have been done prior probing.

	// Check for known memory size at baseAddress.
	// Use a large number of mirror validation because in a lot of save content
	// data is replicated at many addresses which would cause false positive result
	// for the mirroring test.
	knownSizes := []int{
		32 * 1024,  // SRAM (256Kib)
		96 * 1024,  // Dezaemon 3D SRAM (768Kib)
		128 * 1024, // Flash (1Mib)
	}
	size, err = probeDeviceForKnownSizes(r, baseAddress, knownSizes, 5, 2000, 32*1024, l)
	if err != nil {
		return ProbedDevice{}, err
	}

	// FIXME?: For now, assume that we can distinguish SRAM vs Flash based on size.
	// This is true for officially released cartridges but may be wrong in the future.
	switch size {
	case 32 * 1024, 96 * 1024:
		return ProbedDevice{
			Device:      SRAM,
			BaseAddress: baseAddress,
			Size:        size,
		}, nil
	case 128 * 1024:
		return ProbedDevice{
			Device:      FlashRAM,
			BaseAddress: baseAddress,
			Size:        size,
		}, nil
	}

	return ProbedDevice{}, ErrSizeProbeFailure
}
