package pi

import (
	"errors"
)

var (
	// Implementors of ParallelInterface may return
	// this error if the requested operation is not supported.
	ErrUnsupported = errors.New("operation is not supported")
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
