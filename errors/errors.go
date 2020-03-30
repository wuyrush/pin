package errors

import (
	"errors"
	"net/http"
	"strings"
)

type ErrCode string

const (
	ErrCodeNotImplemented    ErrCode = "NotImplemented"
	ErrCodeNotFound          ErrCode = "NotFound"
	ErrCodeServiceFailure    ErrCode = "ServiceFailure"
	ErrCodeAPIBadRequest     ErrCode = "BadRequest"
	ErrCodeDependencyFailure ErrCode = "DepedencyFailure"
	ErrCodeExisted           ErrCode = "Existed"
)

type PinErr struct {
	Code  ErrCode
	msg   string
	cause error
}

func (e *PinErr) Error() string {
	return e.msg
}

// Trace returns the stacktrace associated with the error
func (e *PinErr) Trace() string {
	b := &strings.Builder{}
	b.WriteString(e.msg)
	err := errors.Unwrap(e)
	for err != nil {
		b.WriteString("\nCaused by: ")
		b.WriteString(err.Error())
		err = errors.Unwrap(err)
	}
	return b.String()
}

func (e *PinErr) Unwrap() error {
	return e.cause
}

func newPinErr(m string) *PinErr {
	return &PinErr{msg: m}
}

func (e *PinErr) WithCause(c error) *PinErr {
	e.cause = c
	return e
}

// prefer appSpecificErr(msg) over appSpecificErr(msg, cause) since the latter's method signature has less
// readability - user needs to look up docs to know the 2nd param is for cause, while the first one can use
// WithCause() to be explicit
func ErrServiceFailure(m string) *PinErr {
	return &PinErr{
		Code: ErrCodeServiceFailure,
		msg:  m,
	}
}

func ErrNotFound(m string) *PinErr {
	return &PinErr{
		Code: ErrCodeNotFound,
		msg:  m,
	}
}

func ErrBadInput(m string) *PinErr {
	return &PinErr{
		Code: ErrCodeAPIBadRequest,
		msg:  m,
	}
}

func ErrNotImplemented() *PinErr {
	return &PinErr{
		Code: ErrCodeNotImplemented,
		msg:  "Not implemented",
	}
}

func ErrExisted(m string) *PinErr {
	return &PinErr{
		Code: ErrCodeExisted,
		msg:  m,
	}
}

// StatusCode returns the http response status code associated with the PinErr value
func (e *PinErr) StatusCode() int {
	switch e.Code {
	case ErrCodeNotFound:
		return http.StatusNotFound
	case ErrCodeAPIBadRequest:
		return http.StatusBadRequest
	case ErrCodeExisted:
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}
