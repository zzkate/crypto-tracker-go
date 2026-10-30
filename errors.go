package main

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrSyncInProgress = errors.New("sync in progress")
)

// isUniqueViolation определяет нарушение UNIQUE-ограничения.
// В PostgreSQL это SQLSTATE 23505 (unique_violation).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
