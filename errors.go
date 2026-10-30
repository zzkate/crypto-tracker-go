package main

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrSyncInProgress = errors.New("sync in progress")
)

// isUniqueViolation определяет нарушение UNIQUE-ограничения: через перевод GORM
// (gorm.ErrDuplicatedKey) или SQLSTATE 23505 (unique_violation).
func isUniqueViolation(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
