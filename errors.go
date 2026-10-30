package main

import (
	"errors"
	"strings"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrSyncInProgress = errors.New("sync in progress")
)

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
