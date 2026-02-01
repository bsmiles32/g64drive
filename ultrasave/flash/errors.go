package flash

import (
	"errors"
)

var (
	ErrControllerRequirementsNotMet = errors.New("controller requirements not met")
	ErrInvalidOffsetSize            = errors.New("invalid offset / size")
	ErrInvalidPage                  = errors.New("invalid page")
	ErrInvalidPageRange             = errors.New("invalid page range")
	ErrInvalidPageSize              = errors.New("invalid page size")
	ErrOperationNOK                 = errors.New("operation NOK")
)
