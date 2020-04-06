package errors

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorsTrace(t *testing.T) {
	tcs := []struct {
		name     string
		err      *Err
		expected string
	}{
		{
			name:     "ErrWithoutCause",
			err:      NewNotImplemented(),
			expected: "Not implemented",
		},
		{
			name: "ErrWithCauses",
			err: &Err{
				msg: "foo",
				cause: &Err{
					msg:   "bar",
					cause: &Err{msg: "qux"},
				},
			},
			expected: "foo\n\tCaused by: bar\n\t\tCaused by: qux",
		},
	}
	for _, c := range tcs {
		t.Run(c.name, func(t *testing.T) {
			actual := c.err.Trace()
			assert.Equal(t, c.expected, actual, "unexpected error trace")
		})
	}
}

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
	}
	for _, c := range tcs {
		code := c.err.StatusCode()
		assert.Equal(t, c.expectedCode, code, "unexpected status code")
	}
}
