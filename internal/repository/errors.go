package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// PostgreSQL error codes we care about.
const (
	codeUniqueViolation     = "23505"
	codeForeignKeyViolation = "23503"
	codeCheckViolation      = "23514"
)

// pgErrorCode returns the SQLSTATE code, or "" if err is not a PgError.
func pgErrorCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// constraintName returns the violated constraint name, or "".
func constraintName(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.ConstraintName
	}
	return ""
}

// isUniqueViolation reports whether err is a unique constraint violation on
// the named constraint.
func isUniqueViolation(err error, constraint string) bool {
	return pgErrorCode(err) == codeUniqueViolation && constraintName(err) == constraint
}
