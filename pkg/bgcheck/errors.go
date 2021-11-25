package bgcheck

import (
	"errors"
	"fmt"
)

type statusError struct {
	err  error
	code int
}

func NewStatusError(code int, body []byte) error {
	if len(body) > 0 {
		return statusError{err: errors.New(string(body)), code: code}
	}
	return statusError{err: errors.New("no error specified"), code: code}
}

func NewStatusErrorf(code int, f string, v ...interface{}) error {
	s := fmt.Sprintf(f, v...)
	return NewStatusError(code, []byte(s))
}

func (sr statusError) Error() string {
	return fmt.Sprintf("status code %d: %s", sr.code, sr.err)
}
