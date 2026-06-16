package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// PostgreSQL error codes we care about.
// 我们关注的 PostgreSQL 错误码。
const (
	codeUniqueViolation = "23505"
	codeCheckViolation  = "23514"
)

// pgErrorCode returns the SQLSTATE code, or "" if err is not a PgError.
// pgErrorCode 返回 SQLSTATE 错误码；若 err 不是 PgError 则返回 ""。
func pgErrorCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// constraintName returns the violated constraint name, or "".
// constraintName 返回被违反的约束名；没有则返回 ""。
func constraintName(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.ConstraintName
	}
	return ""
}

// isUniqueViolation reports whether err is a unique constraint violation on
// the named constraint.
// isUniqueViolation 判断 err 是否为指定约束上的唯一约束冲突。
func isUniqueViolation(err error, constraint string) bool {
	return pgErrorCode(err) == codeUniqueViolation && constraintName(err) == constraint
}
