package middleware

import (
	"net/http"

	hr "github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

// PanicRecoverer recovers from panic of underlying handlers
func PanicRecoverer() Middleware {
	return func(h hr.Handle) hr.Handle {
		return func(w http.ResponseWriter, r *http.Request, p hr.Params) {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panicReason", r).Error("got panic from underlying handler")
				}
			}()
			h(w, r, p)
		}
	}
}

// RateLimiter limits underlying handler call rate with given token bucket config
// TODO IMO we don't need to reinvent the wheel here; gorilla may already got this
func RateLimiter(burst int, rate float64) Middleware {
	return func(h hr.Handle) hr.Handle {
		return h
	}
}

// HSTSer enforces clients to use HTTPS for interaction with service
// TODO IMO we don't need to reinvent the wheel here; gorilla may already got this
func HSTSer() Middleware {
	return func(h hr.Handle) hr.Handle {
		return h
	}
}

type Middleware func(hr.Handle) hr.Handle

// Chain composites given handler and middlewares
func Chain(h hr.Handle, ms ...Middleware) hr.Handle {
	for _, m := range ms {
		h = m(h)
	}
	return h
}
