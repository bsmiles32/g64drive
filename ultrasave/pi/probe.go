package pi

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"

	"github.com/rasky/g64drive/ultrasave/logger"
)

var (
	ErrSizeProbeFailure = errors.New("unable to probe device size")
)

func StrByteSize(size int) string {
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
// * doesn't work with some repro cart saves (or more precisely lack of save) as they don't necessarily report open bus values
// * doesn't work when data is indistinguishable from mirroring
// FIXME: API is misleading because it doesn't really test that given sizes are the real device size
// just that addresses after the tested address returns open-bus / mirroring values.
func ProbeDeviceForKnownSizes(r WordReaderAt, baseAddress Address, knownSizes []int, openBusValidation, mirrorValidaton, maxRandOffset int, l logger.Logger) (int, error) {
	// Ensure that size tests are done in increasing order
	// so that we report the smallest knownSize that exhibit mirroring / open bus.
	sort.Ints(knownSizes)
	for _, s := range knownSizes {

		// Reset validations counters
		nOpenBus := openBusValidation
		nMirror := mirrorValidaton

		logger.Log(l, "Probing for size: %s\n", StrByteSize(s))
		for {
			offset := rand.Intn(maxRandOffset)

			// Check for open-bus behavior
			a := baseAddress + Address(s+offset)
			openBus := uint32(uint16(a))<<16 | uint32(uint16(a))
			logger.Log(l, "Probing @%08x", a)
			w, err := r.ReadWordAt(a)
			if err != nil {
				logger.Log(l, ": %s\n", err)
				return 0, err
			}
			logger.Log(l, "=%08x", w)

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

type ProbeController interface {
	WordReaderAt
	BurstReaderAt
}

// TODO?: might want to return a pi.Device interface (instead of any) depending on what we want to do with probe result
type DeviceFactoryFunc func(controller interface{}, baseAddress Address, size int) (interface{}, error)
type DeviceProbeFunc func(c ProbeController, baseAddress Address, l logger.Logger) (bool, int, error)

type DeviceClass struct {
	Name     string
	Factory  DeviceFactoryFunc
	Priority int
	Probe    DeviceProbeFunc
}

func RegisterDeviceClass(d DeviceClass) {
	registeredClasses = append(registeredClasses, d)
	sort.SliceStable(registeredClasses, func(i, j int) bool { return registeredClasses[i].Priority < registeredClasses[j].Priority })
}

var (
	registeredClasses = []DeviceClass{}
)

func ProbeDevice(c ProbeController, baseAddress, baseAddress0 Address, l logger.Logger) (string, int, *DeviceFactoryFunc, error) {
	// Always try open-bus / mirroring vs baseAddress0 first to ensure that a real device is present
	// The mirroring test is useful for multi-non-contiguous-chip configuration (like Dezaemon 3D 3x32KiB SRAM).
	if match, size, err := probeForNoDevice(c, baseAddress, baseAddress0, l); err != nil {
		return "", 0, nil, err
	} else if match {
		return "None", size, nil, nil
	}

	for _, d := range registeredClasses {
		if match, size, err := d.Probe(c, baseAddress, l); err != nil {
			return "", 0, nil, err
		} else if match {
			return d.Name, size, &d.Factory, nil
		}
	}

	return "Unknown", 0, nil, nil
}

func probeForNoDevice(c ProbeController, baseAddress, baseAddress0 Address, l logger.Logger) (bool, int, error) {
	logger.Log(l, "Probing for open-bus/mirroring values at %08x vs %08x\n", baseAddress, baseAddress0)

	deltaBase := int(baseAddress - baseAddress0)

	size, err := ProbeDeviceForKnownSizes(c, baseAddress0, []int{0 + deltaBase}, 5, 4*1024, 32*1024, l)
	if err != nil && !errors.Is(err, ErrSizeProbeFailure) {
		return false, 0, err
	}
	if size == deltaBase && !errors.Is(err, ErrSizeProbeFailure) {
		// Assume that size is not meaningful when no device is present
		return true, 0, nil
	}

	// Not conclusive
	return false, 0, nil
}
