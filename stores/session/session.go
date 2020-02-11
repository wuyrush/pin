package session

import (
	"net/http"

	"github.com/go-redis/redis"
	"github.com/gorilla/sessions"
)

/*
Technically we realized Redis-backed implementation of gorilla session store available on the Web is not good for production use;

But on the work item / task arrangement perspective, we can observe following:
While our current situation(as of 02/09/2020) is:
- we yet have a session-backed auth workflow
- gorilla already provides a simple session store implementation for us to craft the auth workflow
	- note the session store impl is based on cookie, aka it stores all the session info into a cookie of incoming request. This means the session details will end up on client side once the request is processed. Therefore it is not very secure to any clients which are sophisticated enough.
- gorilla already provides a sessions.Store interface so that we implement the auth workflow using gorilla sessions and our own session store implementation independently.

So that if we have two devs then we can put one implement the auth workflow and the other implements the session store implementation. This is how we distribute work to the team - this is the basic skill for me to become a more mature engineer and team player
*/

// Redistore is a github.com/gorilla/sessions.Store
// TODO: implement
type Redistore struct {
	DB *redis.Client
}

// Get should return a cached session.
func (s *Redistore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return nil, nil
}

// New should create and return a new session.
//
// Note that New should never return a nil session, even in the case of
// an error if using the Registry infrastructure to cache the session.
func (s *Redistore) New(r *http.Request, name string) (*sessions.Session, error) {
	return nil, nil
}

// Save should persist session to the underlying store implementation.
func (s *Redistore) Save(r *http.Request, w http.ResponseWriter, sess *sessions.Session) error {
	return nil
}
