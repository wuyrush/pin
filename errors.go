package main

import (
	"errors"
	"strings"
)

type errCode string

const (
	errCodeNotImplemented errCode = "NotImplemented"
	errCodeNotFound       errCode = "NotFound"
	errCodeServiceFailure errCode = "ServiceFailure"
	errCodeAPIBadRequest  errCode = "BadRequest"
)

type pinErr struct {
	Code  errCode
	msg   string
	cause error
}

func (e *pinErr) Error() string {
	return e.msg
}

// Trace returns the stacktrace associated with the error
func (e *pinErr) Trace() string {
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

func (e *pinErr) Unwrap() error {
	return e.cause
}

func newPinErr(m string) *pinErr {
	return &pinErr{msg: m}
}

func (e *pinErr) WithCause(c error) *pinErr {
	e.cause = c
	return e
}

// prefer appSpecificErr(msg) over appSpecificErr(msg, cause) since the latter's method signature has less
// readability - user needs to look up docs to know the 2nd param is for cause, while the first one can use
// WithCause() to be explicit
func errServiceFailure(m string) *pinErr {
	return &pinErr{
		Code: errCodeServiceFailure,
		msg:  m,
	}
}
func errNotFound(m string) *pinErr {
	return &pinErr{
		Code: errCodeNotFound,
		msg:  m,
	}
}

func errBadRequest(m string) *pinErr {
	return &pinErr{
		Code: errCodeAPIBadRequest,
		msg:  m,
	}
}

func errNotImplemented() *pinErr {
	return &pinErr{
		Code: errCodeNotImplemented,
		msg:  "Not implemented",
	}
}
