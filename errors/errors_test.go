package errors

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorsStatusCode(t *testing.T) {
	tcs := []struct {
		err          *Err
		expectedCode int
	}{
		{
			err:          NewServiceFailure("fake"),
			expectedCode: http.StatusInternalServerError,
		},
		{
			err:          NewNotFound("fake"),
			expectedCode: http.StatusNotFound,
		},
		{
			err:          NewBadInput("fake"),
			expectedCode: http.StatusBadRequest,
		},
		{
			err:          NewSpam(),
			expectedCode: http.StatusForbidden,
		},
		{
			err:          NewOversized(),
			expectedCode: http.StatusBadRequest,
		},
	}
	for _, c := range tcs {
		code := c.err.StatusCode()
		assert.Equal(t, c.expectedCode, code, "unexpected status code")
	}
}

func TestErrorsStringer(t *testing.T) {
	tcs := []struct {
		err *Err
		exp string
	}{
		{
			err: NewServiceFailure("boom").WithCause(errors.New("fake")),
			exp: "ServiceFailure: boom Caused by: fake",
		},
		{
			err: NewServiceFailure("boom"),
			exp: "ServiceFailure: boom",
		},
		{
			err: NewSpam(),
			exp: "SpamDetected",
		},
	}
	for _, c := range tcs {
		actual := c.err.Error()
		assert.Equal(t, c.exp, actual)
	}
}
