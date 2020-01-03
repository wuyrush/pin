package main

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

func (e *pinErr) Error() string {
	return e.msg
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
