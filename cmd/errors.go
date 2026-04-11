package cmd

import "fmt"

// ExitError wraps an error with a specific exit code.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

// ExitCode extracts the exit code from an ExitError, if any.
func ExitCode(err error) (int, bool) {
	if e, ok := err.(*ExitError); ok {
		return e.Code, true
	}
	return 0, false
}

// ExitCodeError creates an ExitError with the given code and message.
func ExitCodeError(code int, format string, args ...any) error {
	return &ExitError{Code: code, Err: fmt.Errorf(format, args...)}
}
