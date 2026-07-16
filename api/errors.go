package api

import (
	"errors"
	"fmt"
)

// ErrorCode represents a category of API error.
type ErrorCode int

const (
	// ErrInternal is for unexpected internal errors.
	ErrInternal ErrorCode = iota + 1
	// ErrNotFound is for missing tables, columns, or versions.
	ErrNotFound
	// ErrConflict is for version conflicts or concurrent modification.
	ErrConflict
	// ErrInvalidArg is for invalid user-provided arguments.
	ErrInvalidArg
	// ErrStorage is for underlying storage I/O failures.
	ErrStorage
	// ErrClosed is for operations on a closed Database or Table.
	ErrClosed
)

var codeNames = map[ErrorCode]string{
	ErrInternal:   "internal",
	ErrNotFound:   "not_found",
	ErrConflict:   "conflict",
	ErrInvalidArg: "invalid_argument",
	ErrStorage:    "storage_error",
	ErrClosed:     "closed",
}

func (c ErrorCode) String() string {
	if name, ok := codeNames[c]; ok {
		return name
	}
	return fmt.Sprintf("unknown(%d)", c)
}

// Error is the structured API error type.
// It carries a code, human-readable message, the operation name,
// and an optional wrapped cause (the underlying error).
type Error struct {
	Code    ErrorCode
	Message string
	Op      string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s [code=%s] (cause: %v)", e.Op, e.Message, e.Code, e.Err)
	}
	return fmt.Sprintf("%s: %s [code=%s]", e.Op, e.Message, e.Code)
}

func (e *Error) Unwrap() error { return e.Err }

// UserMessage returns a user-friendly message suitable for end-user display.
// It strips internal details while preserving actionable information.
func (e *Error) UserMessage() string {
	switch e.Code {
	case ErrNotFound:
		return fmt.Sprintf("%s: the requested resource was not found. Please verify the table or column name.", e.Message)
	case ErrConflict:
		return "the operation encountered a version conflict. Please retry."
	case ErrInvalidArg:
		return fmt.Sprintf("invalid argument: %s", e.Message)
	case ErrStorage:
		return "a storage error occurred. Please check that the data directory is accessible and has sufficient space."
	case ErrClosed:
		return "the database or table is closed. Please reconnect."
	default:
		return "an unexpected error occurred. Please check the logs for details."
	}
}

// e creates a new API error with wrapping.
func e(code ErrorCode, op, msg string, cause error) *Error {
	return &Error{Code: code, Op: op, Message: msg, Err: cause}
}

// IsCode checks if an error has the given error code.
// It unwraps through the chain to find an api.Error.
func IsCode(err error, code ErrorCode) bool {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr.Code == code
	}
	return false
}

// IsStorageError checks if the error originates from storage I/O.
func IsStorageError(err error) bool {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr.Code == ErrStorage
	}
	return false
}

// sentinel errors for common situations
var (
	ErrDBClosed      = &Error{Code: ErrClosed, Op: "database", Message: "database is closed"}
	ErrTableClosed   = &Error{Code: ErrClosed, Op: "table", Message: "table is closed"}
	ErrTableNotFound = &Error{Code: ErrNotFound, Op: "database", Message: "table not found"}
	ErrWriterStopped = &Error{Code: ErrInternal, Op: "table", Message: "async writer is not running"}
)
