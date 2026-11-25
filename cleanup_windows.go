package main

import (
	"errors"
	"syscall"
)

func retryableCleanupError(err error) bool {
	const (
		sharingViolation syscall.Errno = 32
		lockViolation    syscall.Errno = 33
		deletePending    syscall.Errno = 303
		userMappedFile   syscall.Errno = 1224
	)
	return errors.Is(err, syscall.ERROR_ACCESS_DENIED) ||
		errors.Is(err, sharingViolation) || errors.Is(err, lockViolation) ||
		errors.Is(err, deletePending) || errors.Is(err, userMappedFile) ||
		errors.Is(err, syscall.ERROR_DIR_NOT_EMPTY)
}
