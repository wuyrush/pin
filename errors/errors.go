package errors

import (
	"errors"
	"net/http"
	"strings"
)

// ErrCode provides summary of errors encountered by pin services
type ErrCode string

const (
	ErrCodeNotImplemented    ErrCode = "NotImplemented"
	ErrCodeNotFound          ErrCode = "NotFound"
	ErrCodeServiceFailure    ErrCode = "ServiceFailure"
	ErrCodeAPIBadRequest     ErrCode = "BadRequest"
	ErrCodeDependencyFailure ErrCode = "DepedencyFailure"
	ErrCodeExisted           ErrCode = "Existed"
	ErrCodeSpam              ErrCode = "SpamDetected"
	ErrCodeOversized         ErrCode = "Oversized"
)

// Err models errors encountered by pin services
type Err struct {
	Code  ErrCode
	msg   string
	cause error
}

func (e *Err) Error() string {
	return e.msg
}

// Trace returns the stacktrace associated with the error
func (e *Err) Trace() string {
	b := &strings.Builder{}
	b.WriteString(e.msg)
	err := errors.Unwrap(e)
	indent := ""
	for err != nil {
		b.WriteString("\n")
		indent += "\t"
		b.WriteString(indent)
		b.WriteString("Caused by: ")
		b.WriteString(err.Error())
		err = errors.Unwrap(err)
	}
	return b.String()
}

func (e *Err) Unwrap() error {
	return e.cause
}

// prefer appSpecificErr(msg) over appSpecificErr(msg, cause) since the latter's method signature has less
// readability - user needs to look up docs to know the 2nd param is for cause, while the first one can use
// WithCause() to be explicit
func (e *Err) WithCause(c error) *Err {
	e.cause = c
	return e
}

func (e *Err) WithMsg(m string) *Err {
	e.msg = m
	return e
}

func NewServiceFailure(m string) *Err {
	return &Err{
		Code: ErrCodeServiceFailure,
		msg:  m,
	}
}

func NewNotFound(m string) *Err {
	return &Err{
		Code: ErrCodeNotFound,
		msg:  m,
	}
}

func NewBadInput(m string) *Err {
	return &Err{
		Code: ErrCodeAPIBadRequest,
		msg:  m,
	}
}

func NewNotImplemented() *Err {
	return &Err{
		Code: ErrCodeNotImplemented,
		msg:  "Not implemented",
	}
}

func NewExisted(m string) *Err {
	return &Err{
		Code: ErrCodeExisted,
		msg:  m,
	}
}

func NewSpam() *Err {
	return &Err{Code: ErrCodeSpam}
}

func NewOversized() *Err {
	return &Err{Code: ErrCodeOversized}
}

// StatusCode returns the http response status code associated with the Err value
func (e *Err) StatusCode() int {
	switch e.Code {
	case ErrCodeNotFound:
		return http.StatusNotFound
	case ErrCodeAPIBadRequest, ErrCodeOversized:
		return http.StatusBadRequest
	case ErrCodeSpam:
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}
