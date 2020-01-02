package main

import (
	"github.com/go-pg/pg/v9"
)

// Interface of db used in the application.
// This is to retain the flexibility to adopt multiple kinds of db and switching.
type pinDB interface {
	// get info in text mode given its id
	GetInfoText(string) (*info, error)
	// get info in binary mode given its id
	GetInfoBinary(string) (*info, error)
}

// pgr implements pinDB interface driven by PostgreSQL
type pgr struct {
	db *pg.DB
}

func (p *pgr) GetInfoText(id string) (*info, error) {
	i := &info{ID: id}
	if err := p.db.Select(i); err != nil {
		// TODO: translate to native error type
		return nil, err
	}
	return i, nil
}

// pgr is only used to store info in text mode
func (p *pgr) GetInfoBinary(id string) (*info, error) {
	return nil, errNotImplemented()
}
