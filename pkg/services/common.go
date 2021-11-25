package services

import (
	"fmt"
)

// Error defines an error type.
type Error struct {
	Message string
	Code    int
}

// NewError returns a new Error.
func NewError(message string, code int) *Error {
	return &Error{
		Message: message,
		Code:    code,
	}
}

// NewErrorWrap returns a new Error using the provided error in the message.
func NewErrorWrap(err error, message string, code int) *Error {
	return &Error{
		Message: fmt.Sprintf("%s: %s", err.Error(), message),
		Code:    code,
	}
}

func (e *Error) Error() string {
	return fmt.Sprintf("SERVICE_ERR-%d: %s", e.Code, e.Message)
}
