package known_errors

import "errors"

var (
	ErrBadRequest  = errors.New("bad request")
	ErrNotFound    = errors.New("not found")
	ErrDuplicated  = errors.New("duplicated")
	ErrUnauthorized = errors.New("unauthorized")
)
