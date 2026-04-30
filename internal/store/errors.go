package store

import "errors"

var (
	ErrEmptyKey      = errors.New("key is required")
	ErrWrongType     = errors.New("wrong type for key")
	ErrInvalidValue  = errors.New("invalid value for key type")
	ErrEmptyFieldSet = errors.New("at least one field is required")
	ErrEmptyValues   = errors.New("at least one value is required")
	ErrNotInteger    = errors.New("value is not an integer or out of range")
)
