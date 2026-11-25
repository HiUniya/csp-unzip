//go:build !windows

package main

func retryableCleanupError(error) bool {
	return false
}
