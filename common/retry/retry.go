package retry

import (
	"math"
	"strings"
	"time"
)

/*
	Retry utils with following feature:
	- exponential backoff
	- jitter
	- max attempts
	- max timeout

	Retries up to either MaxAttempts or till Timeout or RetryOn returns false. The time interval between the i-th and (i+1)-th
	attempt is `min( BaseDelay * ( Exp ^ ( i+1 ) + Jitter ), MaxBackoff )`
*/

// Fn is the function to apply retry strategies on
type Fn func() error

// RetryOnFn decides whether to retry on given error
type RetryOnFn func(error) bool

type RetryConfig struct {
	MaxAttempts int64
	MaxBackoff  time.Duration // maximum wait time before next attempt, non-negative value means no wait
	Timeout     time.Duration // non-negative value means timeout immediately
	Jitter      float64
	BaseDelay   time.Duration
	Exp         float64
	RetryOn     RetryOnFn
}

type RetryOption func(*RetryConfig)

func defaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts: math.MaxInt64,
		MaxBackoff:  time.Duration(math.MaxInt64),
		Timeout:     time.Duration(math.MaxInt64),
		Exp:         1,
		RetryOn:     func(error) bool { return false },
	}
}

func WithMaxAttempts(a int64) RetryOption {
	return func(c *RetryConfig) {
		c.MaxAttempts = a
	}
}

func WithTimeout(t time.Duration) RetryOption {
	return func(c *RetryConfig) {
		c.Timeout = t
	}
}

func WithJitter(j float64) RetryOption {
	return func(c *RetryConfig) {
		c.Jitter = j
	}
}

func WithBaseDelay(t time.Duration) RetryOption {
	return func(c *RetryConfig) {
		c.BaseDelay = t
	}
}

func WithExp(e float64) RetryOption {
	return func(c *RetryConfig) {
		c.Exp = e
	}
}

func WithRetryOn(f RetryOnFn) RetryOption {
	return func(c *RetryConfig) {
		c.RetryOn = f
	}
}

func WithMaxBackoff(b time.Duration) RetryOption {
	return func(c *RetryConfig) {
		c.MaxBackoff = b
	}
}

func Retry(f Fn, opts ...RetryOption) error {
	cfg := defaultRetryConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	// fire f first in case it doesn't need retry at all
	err := f()
	if !cfg.RetryOn(err) {
		return err
	}
	// receive from nil chan always block, representing no timeout
	var timeout <-chan time.Time
	if cfg.Timeout != 0 {
		// note that a timer fires immediately if created with a non-positive duration
		t := time.NewTimer(cfg.Timeout)
		defer t.Stop()
		timeout = t.C
	}
	var i int64
	for ; i <= cfg.MaxAttempts; i++ {
		factor := math.Pow(cfg.Exp, float64(i)) + cfg.Jitter
		// cap the delay to the max of time.Duration, which is ~290 years
		delay := time.Duration(math.Min(float64(cfg.BaseDelay.Nanoseconds())*factor, math.MaxInt64)) * time.Nanosecond
		if delay > cfg.MaxBackoff {
			delay = cfg.MaxBackoff
		}
		t := time.NewTimer(delay)
		defer t.Stop()
		select {
		case <-t.C:
			err = f()
			if !cfg.RetryOn(err) {
				return err
			}
		case <-timeout:
			return ErrRetryTimedOut
		}
	}
	return err
}

// ------------- helpers -------------
func IsDepOffline(e error) bool {
	return e != nil && strings.Index(e.Error(), "connect: connection refused") >= 0
}

type errRetry string

func (e errRetry) Error() string {
	return string(e)
}

const ErrRetryTimedOut errRetry = "retry timed out"
