package main

import (
	"io"
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

type infoType int

const (
	text infoType = iota
	binary
)

type accessMode int

const (
	public accessMode = iota
	private
)

type metadata struct {
	ID           string
	OwnerID      *string
	Type         infoType
	Mode         accessMode
	Expiry       time.Time
	ViewCount    uint64
	MaxViewCount *uint64
}

func (m *metadata) AccessibleTo(u *user) bool {
	// TODO: implement
	return false
}

func (m *metadata) Expired() bool {
	// TODO: implement
	return false
}

type info struct {
	ID   string
	Body io.ReadCloser // prefer not to promptly load the actual content into memory
}
