package kafkautil

import (
	"errors"
	"io"
	"syscall"
)

func IsTemporaryError(err error) bool {
	var tempError interface{ Temporary() bool }
	if errors.As(err, &tempError) {
		return tempError.Temporary()
	}
	return false
}

func IsTransientNetworkError(err error) bool {
	return errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE)
}
