package models

import (
	"time"

	cst "wuyrush.io/pin/constants"
)

/*
 Application layer data models.
*/

type AccessMode int

const (
	AccessModePublic AccessMode = iota
	AccessModePrivate
)

var AccessModeVals = map[AccessMode]struct{}{
	AccessModePublic:  {},
	AccessModePrivate: {},
}

type Pin struct {
	ID           string
	OwnerID      string
	Mode         AccessMode
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

func (p *Pin) VisibleTo(u *User) bool {
	// TODO: implement
	return false
}

// Checks if the pin info is expired or not. A pin info is expired if and only if
// 1. The current server time is later than the pin info's expiry OR
// 2. The pin info has ReadAndBurn marked as true and ViewCount >= 1
// The application shall remove all expired pin info from cache to prevent any further access.
func (p *Pin) Expired() bool {
	return time.Now().After(p.CreationTime.Add(p.GoodFor)) || (p.ReadAndBurn && p.ViewCount >= 1)
}

// pinView vends necessary pin data for rendering web pages
type PinView struct {
	Pin
	Expiry        time.Time
	Err           string
	URL           string
	FilenameToURL map[string]string
}

// Junk represents necessary pin data for deletion purpose
type Junk struct {
	PinID    string   // pin ID
	FileRefs []string // references of pin's attachments on storage layer
}

// User models individual service user
type User struct {
	ID           string
	IDType       cst.IDType
	Passwd       string // only used during email / phone registration. Ignored in all other scenarios
	Hash         string
	CreationTime time.Time
	Active       bool
}

func (u *User) Anonymous() bool {
	return u == nil
}
