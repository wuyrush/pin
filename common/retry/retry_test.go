package retry

import (
	"fmt"
	"testing"
)

type testErrRetryable struct {
}

func (e testErrRetryable) Error() string {
	return "retryable err"
}

func TestRetry(t *testing.T) {
	retryable, nonRetryable := testErrRetryable{}, fmt.Errorf("non-retryable")
	f := func(count *int, errs []error) error {
		cnt := *count
		// to prove the function logic is actually executed
		*count = cnt + 1
		return errs[cnt]
	}
	retryOn := func(e error) bool {
		_, ok := e.(testErrRetryable)
		return ok
	}
	tcs := []struct {
		name     string
		errs     []error
		strategy []RetryOption
		expected int
	}{
		{
			name:     "no retry",
			errs:     []error{nil},
			expected: 1,
		},
		{
			name: "retry with max attempt",
			errs: []error{
				retryable,
				retryable,
				nonRetryable,
			},
			expected: 3,
			strategy: []RetryOption{
				WithMaxAttempts(2),
				WithRetryOn(retryOn),
			},
		},
		{
			name: "retryOn",
			errs: []error{
				retryable,
				retryable,
				nonRetryable,
				retryable,
				retryable,
			},
			expected: 3,
			strategy: []RetryOption{
				WithMaxAttempts(10),
				WithRetryOn(retryOn),
			},
		},
		// TODO: test timeout and exponential backoff
	}

	for _, c := range tcs {
		errs, strategy, exp := c.errs, c.strategy, c.expected
		t.Run(c.name, func(*testing.T) {
			actual := 0
			Retry(
				func() error {
					// f can also return result besides values as long as we refer to
					// the result with pointer so that it won't get lost
					return f(&actual, errs)
				},
				strategy...,
			)
			if actual != exp {
				t.Errorf("expected %d for %v and %v but got %d", exp, errs, strategy, actual)
			}
		})
	}

}
