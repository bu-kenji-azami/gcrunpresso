package gcrunpresso

import (
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ErrSkipVerify string

func (e ErrSkipVerify) Error() string {
	return string(e)
}

type ErrNotFound string

func (e ErrNotFound) Error() string {
	return string(e)
}

type ErrConflictOptions string

func (e ErrConflictOptions) Error() string {
	return string(e)
}

type ErrPermissionDenied string

func (e ErrPermissionDenied) Error() string {
	return string(e)
}

type ExitCodeError struct {
	Code int
	Err  error
}

func (e *ExitCodeError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit code %d", e.Code)
}

func (e *ExitCodeError) ExitCode() int {
	return e.Code
}

func (e *ExitCodeError) Unwrap() error {
	return e.Err
}

var (
	errNotFound         = ErrNotFound("not found")
	errSkipVerify       = ErrSkipVerify("skip verify")
	errPermissionDenied = ErrPermissionDenied("permission denied")
)

func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	if st, ok := status.FromError(err); ok {
		return st.Code() == codes.PermissionDenied || st.Code() == codes.Unauthenticated
	}
	var gErr *googleapi.Error
	if errors.As(err, &gErr) {
		return gErr.Code == http.StatusForbidden || gErr.Code == http.StatusUnauthorized
	}
	return false
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	if st, ok := status.FromError(err); ok {
		return st.Code() == codes.NotFound
	}
	var gErr *googleapi.Error
	if errors.As(err, &gErr) {
		return gErr.Code == http.StatusNotFound
	}
	return false
}

func wrapPermissionError(err error) error {
	if err == nil {
		return nil
	}
	if isPermissionError(err) {
		return ErrPermissionDenied(err.Error())
	}
	return err
}
