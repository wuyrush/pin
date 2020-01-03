package main

import (
	"time"

	log "github.com/sirupsen/logrus"
)

// Formatter to log the unix time in milliseconds.
type unixTimeFormatter struct {
	log.Formatter
}

// I've noticed passing a mutated *log.Entry value to downstream formatter results in logs with panic level
// and empty message, but never sure about why it happens
func (f *unixTimeFormatter) Format(e *log.Entry) ([]byte, error) {
	e.Data["epochTimeMillis"] = e.Time.UnixNano() / int64(time.Millisecond)
	return f.Formatter.Format(e)
}
