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
)

type pinErr struct {
	code     errCode
	msg      string
	onClient bool
	cause    error
}

// prefer stacktrace due to prevalence of nested errors
func (e *pinErr) Error() string {
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

// prefer incremental building the error up than explicit instantiation via pointer(e := &pinErr{...})
func (e *pinErr) WithOnClient(b bool) *pinErr {
	e.onClient = b
	return e
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
		code: errCodeServiceFailure,
		msg:  m,
	}
}
func errNotFound(m string) *pinErr {
	return &pinErr{
		code: errCodeNotFound,
		msg:  m,
	}
}

func errNotImplemented() *pinErr {
	return &pinErr{
		code: errCodeNotImplemented,
		msg:  "Not implemented",
	}
}
