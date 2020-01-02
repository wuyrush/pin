package main

import (
	"fmt"
	"time"

	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
)

const (
	fieldNameOwnerID  = "ownerId"
	fieldNameType     = "type"
	fieldNameMode     = "mode"
	fieldViewCount    = "viewCount"
	fieldMaxViewCount = "maxViewCount"
	fieldNameExpiry   = "expiry"
)

// Interface of all caches used in the application.
// This is to retain the flexibility to adopt multiple kinds of cache and switching.
type pinCache interface {
	PutMetadata(*metadata) error
	GetMetadata(id string) (*metadata, error)
}

// pinRedis implements pinCache interface driven by Redis
type pinRedis struct {
	db *redis.Client
}

func (r *pinRedis) PutMetadata(m *metadata) error {
	cl := log.WithFields(log.Fields{"id": m.id, "expiry": m.Expiry})
	_, err := r.db.HMSet(m.ID, map[string]interface{}{
		fieldNameOwnerID:  m.OwnerID,
		fieldNameType:     m.Type,
		fieldNameMode:     m.Mode,
		fieldViewCount:    m.ViewCount,
		fieldMaxViewCount: m.MaxViewCount,
		fieldNameExpiry:   m.Expiry,
	}).Result()
	if err != nil {
		cl.WithError(err).Error("failed saving metadata")
		return errServiceFailure("failed saving metadata").WithCaus(err)
	}
	// set expiry. TODO: janitor mechanism to cleanup possible leaked metadata
	_, err = r.db.ExpireAt(m.ID, m.Expiry).Result()
	if err != nil {
		cl.WithError(err).Error("failed to set metadata expiry. Extra cleanup needed")
		return errServiceFailure("failed to set metadata expiry").WithCause(err)
	}
	return err
}

func (r *pinRedis) GetMetadata(id string) (*metadata, error) {
	s, err := r.db.HMGet(id,
		"fieldNameOwnerID",
		"fieldNameType",
		"fieldNameMode",
		"fieldNameExpiry",
		"fieldViewCount",
		"fieldMaxViewCountwCount",
	).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, errNotFound(fmt.Sprintf("metadata not found in cache")).WithCause(err)
		}
		errStr := "failed getting metadata"
		log.WithError(err).WithField("id", id).Error(errStr)
		return nil, errServiceFailure(errStr).WithCause(err)
	}
	return &metadata{
		ID:           id,
		OwnerID:      s[0].(*string),
		Type:         s[1].(infoType),
		Mode:         s[2].(accessMode),
		Expiry:       s[3].(time.Time),
		ViewCount:    s[4].(uint64),
		MaxViewCount: s[5].(*uint64),
	}, nil
}
