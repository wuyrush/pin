package models

import (
	"time"
)

// AccessMode describes a pin's accessibility
type AccessMode uint8

const (
	AccessModePublic AccessMode = iota
	AccessModePrivate
)

var accessModes = [...]string{
	"Public",
	"Private",
}

func (m AccessMode) String() string {
	return accessModes[uint8(m)]
}

// Pin models the information pinned by users
type Pin struct {
	ID           string // ID is also a Pin's URI
	OwnerID      string
	AccessMode   AccessMode
	CreationTime time.Time
	GoodFor      time.Duration
	ReadAndBurn  bool
	Title        string
	Body         string
	// Attachments stores mappings between attachment's url-encoded filename and
	// its reference in file storage layer
	Attachments map[string]string
	// 	a pin's view count need to be updated frequently as more and more user read the same pin
	// so not a good idea to store it as part of the pin document if we use a document-based db
	// otherwise we will see increasing amount of db updates just for changing a single field
	// Therefore a better place to maintain such state IMO is in cache
}

func (p *Pin) VisibleTo(u *User) bool {
	return p.AccessMode == AccessModePublic || (!u.Anonymous() && u.ID == p.OwnerID)
}

// Stale tells if a pin should be removed or not.
func (p *Pin) Stale() bool {
	return p.CreationTime.Add(p.GoodFor).Before(time.Now())
}

// User models individual service user
type User struct {
	ID           string
	IDType       UserIDType
	Passwd       string // only used during registration
	Hash         string
	CreationTime time.Time
	Active       bool
}

func (u *User) Anonymous() bool {
	return u == nil
}

// IDType represents user ID type
type UserIDType uint8

const (
	UserIDTypeInvalid UserIDType = iota
	UserIDTypeEmail
	UserIDTypePhoneNumber
)

var idTypes = [...]string{
	"InvalidUserIDType",
	"Email",
	"PhoneNumber",
}

func (t UserIDType) String() string {
	idx := int(t)
	if idx >= len(idTypes) {
		return idTypes[0]
	}
	return idTypes[idx]
}
