package errs

import "errors"

var (
	ErrNotFound     = errors.New("not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrConflict     = errors.New("conflict")
	ErrForbidden    = errors.New("forbidden")
)
