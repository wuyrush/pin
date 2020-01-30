package main

import (
	"math"
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

// fn is the function to retry
type fn func() error

// retryOnFn decides whether to retry on given error
type retryOnFn func(error) bool

type retryConfig struct {
	MaxAttempts int64
	MaxBackoff  time.Duration // maximum wait time before next attempt, non-negative value means no wait
	Timeout     time.Duration // non-negative value means timeout immediately
	Jitter      float64
	BaseDelay   time.Duration
	Exp         float64
	RetryOn     retryOnFn
}

type retryOption func(*retryConfig)

func defaultRetryConfig() *retryConfig {
	return &retryConfig{
		MaxAttempts: math.MaxInt64,
		MaxBackoff:  time.Duration(math.MaxInt64),
		Timeout:     time.Duration(math.MaxInt64),
		Exp:         1,
		RetryOn:     func(error) bool { return false },
	}
}

func withMaxAttempts(a int64) retryOption {
	return func(c *retryConfig) {
		c.MaxAttempts = a
	}
}

func withTimeout(t time.Duration) retryOption {
	return func(c *retryConfig) {
		c.Timeout = t
	}
}

func withJitter(j float64) retryOption {
	return func(c *retryConfig) {
		c.Jitter = j
	}
}

func withBaseDelay(t time.Duration) retryOption {
	return func(c *retryConfig) {
		c.BaseDelay = t
	}
}

func withExp(e float64) retryOption {
	return func(c *retryConfig) {
		c.Exp = e
	}
}

func withRetryOn(f retryOnFn) retryOption {
	return func(c *retryConfig) {
		c.RetryOn = f
	}
}

func withMaxBackoff(b time.Duration) retryOption {
	return func(c *retryConfig) {
		c.MaxBackoff = b
	}
}

func retry(f fn, opts ...retryOption) error {
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
			return errRetryTimedOut
		}
	}
	return err
}

type errRetry string

func (e errRetry) Error() string {
	return string(e)
}

const errRetryTimedOut errRetry = "retry timed out"
