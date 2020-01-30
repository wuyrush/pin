package main

import (
	"time"
)

/*
 Application layer data models.
*/

type user struct {
	ID string
}

func (u *user) Anonymous() bool {
	return u == nil
}

type accessMode int

const (
	accessModePublic accessMode = iota
	accessModePrivate
)

var accessModeVals = map[accessMode]struct{}{
	accessModePublic:  {},
	accessModePrivate: {},
}

type pin struct {
	ID           string
	OwnerID      string
	Mode         accessMode
	CreationTime time.Time
	GoodFor      time.Duration
	ReadAndBurn  bool
	ViewCount    uint64
	Title        string
	Note         string
	// Attachments stores mappings between attachment's url-encoded filename and
	// its reference in file storage layer
	Attachments map[string]string
}

func (p *pin) VisibleTo(u *user) bool {
	// TODO: implement
	return false
}

// Checks if the pin info is expired or not. A pin info is expired if and only if
// 1. The current server time is later than the pin info's expiry OR
// 2. The pin info has ReadAndBurn marked as true and ViewCount >= 1
// The application shall remove all expired pin info from cache to prevent any further access.
func (p *pin) Expired() bool {
	return time.Now().After(p.CreationTime.Add(p.GoodFor)) || (p.ReadAndBurn && p.ViewCount >= 1)
}

/*
{
	pinID: {
		metadata: {

		},
		content: {

		}
	}
}
*/
