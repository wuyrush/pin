package stores

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"wuyrush.io/pin/common/logging"
	cst "wuyrush.io/pin/constants"
	pe "wuyrush.io/pin/errors"
	md "wuyrush.io/pin/models"
)

// PinStore vends the interface to interact with pin data.
type PinStore interface {
	Get(pinID string) (*md.Pin, *pe.PinErr)
	// Register registers pin for bookkeeping purpose
	Register(p *md.Pin) *pe.PinErr
	// Deregister de-register pin from PinStore. Caller must ensure the pin data is all cleaned up before
	// calling Deregister to avoid leaking pin data
	Deregister(pinID string) *pe.PinErr
	Save(p *md.Pin) *pe.PinErr
	// Delete deletes pin data from store. Delete must be idempotent
	Delete(pinID string) *pe.PinErr
	// Junk returns pins which shall be removed from PinStore of size max;
	// It returns all junk pins when max == 0
	Junk(max int) ([]*md.Junk, *pe.PinErr)
	Close() *pe.PinErr
}

// RedisStore is a PinStore implementation driven by Redis.
type RedisStore struct {
	DB *redis.Client
}

const (
	fieldNameOwnerID      = "ownerId"
	fieldNameMode         = "mode"
	fieldNameViewCount    = "viewCount"
	fieldNameCreationTime = "creationTime"
	fieldNameGoodFor      = "goodFor"
	fieldNameReadAndBurn  = "readAndBurn"
	fieldNameTitle        = "title"
	fieldNameNote         = "note"
	fieldNameAttachments  = "attachments"

	// redis key of the sorted set whose score is pin expiry
	keyPinExpirySet = "pinExpirySet"
	// template to form an unique identifier for pin attachment refs
	keyTmplRefs = `refs.%s`
)

func (s *RedisStore) Register(p *md.Pin) *pe.PinErr {
	const errMsg = "error registering pin"
	clog := log.WithField("pinID", p.ID)
	expiry := p.CreationTime.Add(p.GoodFor).Unix()
	// add pin ID to sorted set for future lookup
	member := redis.Z{
		Score:  float64(expiry),
		Member: p.ID,
	}
	if _, err := s.DB.ZAddNX(keyPinExpirySet, member).Result(); err != nil {
		clog.WithError(err).Error("Register: error calling Redis to index pin id")
		return pe.ErrServiceFailure(errMsg).WithCause(err)
	}
	// cache necessary pin data for future cleanup
	refs, cnt := make([]string, len(p.Attachments)), 0
	for _, ref := range p.Attachments {
		refs[cnt] = ref
		cnt++
	}
	refsByte, err := json.Marshal(refs)
	if err != nil {
		clog.WithError(err).Error("Register: error marshalling pin attachment references to json")
		return pe.ErrServiceFailure(errMsg).WithCause(err)
	}
	refsKey := s.refsKey(p.ID)
	if _, err := s.DB.Set(refsKey, refsByte, time.Duration(0)).Result(); err != nil {
		clog.WithError(err).WithField("refsKey", refsKey).Error("Register: error calling Redis to save pin attachment refs")
		return pe.ErrServiceFailure(errMsg).WithCause(err)
	}
	return nil
}

func (s *RedisStore) Deregister(pinID string) *pe.PinErr {
	const errMsg = "error deregistering pin"
	clog := log.WithField("pinID", pinID)
	// remove pin attachment refs data if any
	refsKey := s.refsKey(pinID)
	// redis ignores the error upon DEL if the key is non-existent
	if _, err := s.DB.Del(refsKey).Result(); err != nil {
		clog.WithError(err).WithField("refsKey", refsKey).Error("Deregister: error calling redis to remove pin attachment refs")
		return pe.ErrServiceFailure(errMsg).WithCause(err)
	}
	// remove pin id from index
	// redis ignores the error upon ZREM if the key is non-existent
	if _, err := s.DB.ZRem(keyPinExpirySet, pinID).Result(); err != nil {
		clog.WithError(err).Error("Deregister: error calling redis to remove pin id from index")
		return pe.ErrServiceFailure(errMsg).WithCause(err)
	}
	return nil
}

func (s *RedisStore) Junk(max int) ([]*md.Junk, *pe.PinErr) {
	const errMsg = "error loading junk pins"
	clog := logging.WithFuncName()
	count := max
	if max < 0 {
		return nil, pe.ErrBadInput(fmt.Sprintf("got negative max item count %d", max))
	} else if max == 0 {
		count = -1
	}
	// gather stale pin ids
	now := time.Now().Unix()
	opt := redis.ZRangeBy{Min: "0", Max: strconv.FormatInt(now, 10), Count: int64(count)}
	ids, err := s.DB.ZRangeByScore(keyPinExpirySet, opt).Result()
	if err != nil {
		clog.WithError(err).Error("error calling redis to get ids of stale pins")
		return nil, pe.ErrServiceFailure(errMsg).WithCause(err)
	}
	clog.WithField("ids", ids).Debug("done loading junk pin ids")
	// assemble junk pins and return
	jks, err := s.junk(ids)
	if err != nil {
		return nil, pe.ErrServiceFailure(errMsg).WithCause(err)
	}
	clog.WithField("junks", jks).Debug("done assembling junk pins")
	return jks, nil
}

func (s *RedisStore) junk(ids []string) ([]*md.Junk, error) {
	clog := logging.WithFuncName()
	// this concurrency setup guarantees following ordering: ALL GetPinRefFromRedis goroutine finish ->
	// MakeErrStat goroutine gets the very last err and the goroutine executing junk() gets the last junk ->
	// waiter goroutine unblocks from wait and closes done channel -> MakeErrStat and junk() goroutine exit.
	fpsize := viper.GetInt(cst.EnvPinStoreJunkFetcherPoolSize)
	quotas := make(chan struct{}, fpsize)
	jkChan, errChan, done := make(chan *md.Junk), make(chan error), make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(len(ids))
	// waiter
	go func() {
		wg.Wait()
		close(done)
	}()
	// a dedicated goroutine to collect error stats
	errcnt := 0
	go func() {
		for {
			select {
			case <-errChan:
				errcnt++
			case <-done:
				return
			}
		}
	}()
	// spawn assemblers
	clog.WithField("fetcherPoolSize", fpsize).Debug("spawning fetchers")
	for _, pinID := range ids {
		go func(pinID string) {
			// NOTE individual worker should be responsible for acquiring quota otherwise we risk blocking
			// goroutine executing the enclosing function(in this case `junk()`)
			quotas <- struct{}{}
			defer func() { <-quotas }()
			defer wg.Done()
			refsKey := s.refsKey(pinID)
			refsStr, err := s.DB.Get(refsKey).Result()
			if err != nil {
				clog.WithError(err).WithField("pinID", pinID).Error("error getting pin file refs from redis")
				errChan <- err
				return
			}
			refs := &[]string{}
			if err := json.Unmarshal([]byte(refsStr), refs); err != nil {
				clog.WithError(err).WithField("pinID", pinID).Error("error unmarshal pin file refs")
				errChan <- err
				return
			}
			jkChan <- &md.Junk{PinID: pinID, FileRefs: *refs}
		}(pinID)
	}
	// goroutine executing this function to collect assembled junk pins
	jks := make([]*md.Junk, 0, len(ids))
	for {
		select {
		case jk := <-jkChan:
			jks = append(jks, jk)
		case <-done:
			clog.Debug("done collecting junk pins")
			if errcnt > 0 {
				clog.Errorf("got %d errors when retrieving junk pin file refs from redis. See log before time %s",
					errcnt, time.Now().UTC())
			}
			return jks, nil
		}
	}
}

func (s *RedisStore) refsKey(pinID string) string {
	return fmt.Sprintf(keyTmplRefs, pinID)
}

func (s *RedisStore) Save(p *md.Pin) *pe.PinErr {
	const errMsg = "error saving pin metadata"
	clog := logging.WithFuncName().WithField("pinID", p.ID)
	filesBytes, err := json.Marshal(p.Attachments)
	if err != nil {
		clog.WithError(err).Error("error marshalling pin attachment metadata to JSON")
		return pe.ErrServiceFailure(errMsg).WithCause(err)
	}
	// hacks redis keys so that data in the nested object can be store in a flat map
	if _, err := s.DB.HMSet(p.ID, map[string]interface{}{
		fieldNameOwnerID:      p.OwnerID,
		fieldNameMode:         int(p.Mode),
		fieldNameViewCount:    p.ViewCount,
		fieldNameCreationTime: p.CreationTime,
		fieldNameGoodFor:      int64(p.GoodFor),
		fieldNameReadAndBurn:  p.ReadAndBurn,
		fieldNameTitle:        p.Title,
		fieldNameNote:         p.Note,
		fieldNameAttachments:  filesBytes,
	}).Result(); err != nil {
		clog.WithError(err).Error("error caching pin metadata in redis")
		return pe.ErrServiceFailure(errMsg).WithCause(err)
	}
	if _, err := s.DB.Expire(p.ID, p.GoodFor).Result(); err != nil {
		clog.WithError(err).Error("error setting pin's expiry")
		return pe.ErrServiceFailure(errMsg).WithCause(err)
	}
	return nil
}

func (s *RedisStore) Get(pinID string) (*md.Pin, *pe.PinErr) {
	clog := logging.WithFuncName().WithField("pinID", pinID)
	m, err := s.DB.HGetAll(pinID).Result()
	if err != nil {
		msg := "error getting pin data"
		clog.WithError(err).Error(msg)
		return nil, pe.ErrServiceFailure(msg).WithCause(err)
	}
	// if Redis had expired the pin the API will return an empty map
	if m == nil || len(m) == 0 {
		return nil, pe.ErrNotFound(fmt.Sprintf("pin %s not found", pinID))
	}
	p := &md.Pin{
		ID:      pinID,
		OwnerID: m[fieldNameOwnerID],
		Title:   m[fieldNameTitle],
		Note:    m[fieldNameNote],
	}
	mode, err := strconv.Atoi(m[fieldNameMode])
	if err != nil {
		msg := "error unmarshalling access mode"
		clog.WithError(err).Error(msg)
		return nil, pe.ErrServiceFailure(msg).WithCause(err)
	}
	p.Mode = md.AccessMode(mode)

	vc, err := strconv.ParseInt(m[fieldNameViewCount], 10, 64)
	if err != nil {
		msg := "error unmarshalling view count"
		clog.WithError(err).Error(msg)
		return nil, pe.ErrServiceFailure(msg).WithCause(err)
	}
	p.ViewCount = uint64(vc)

	var t time.Time
	if err := t.UnmarshalBinary([]byte(m[fieldNameCreationTime])); err != nil {
		msg := "error unmarshalling pin creation time"
		clog.WithError(err).Error(msg)
		return nil, pe.ErrServiceFailure(msg).WithCause(err)
	}
	p.CreationTime = t

	gf, err := strconv.ParseInt(m[fieldNameGoodFor], 10, 64)
	if err != nil {
		msg := "error unmarshalling good-for period"
		clog.WithError(err).Error(msg)
		return nil, pe.ErrServiceFailure(msg).WithCause(err)
	}
	p.GoodFor = time.Duration(gf)

	attachments := make(map[string]string)
	if m[fieldNameAttachments] != "" {
		if err := json.Unmarshal([]byte(m[fieldNameAttachments]), &attachments); err != nil {
			msg := "error unmarshalling pin attachment metadata"
			clog.WithError(err).Error(msg)
			return nil, pe.ErrServiceFailure(msg).WithCause(err)
		}
	}
	p.Attachments = attachments
	return p, nil
}

func (s *RedisStore) Delete(pinID string) *pe.PinErr {
	clog := logging.WithFuncName().WithField("pinID", pinID)
	// 1. attempt removing pin (meta)data
	if _, err := s.DB.Del(pinID).Result(); err != nil && err != redis.Nil {
		msg := "error deleting pin data from Redis"
		clog.WithError(err).Error(msg)
		return pe.ErrServiceFailure(msg).WithCause(err)
	}
	return nil
}

func (s *RedisStore) Close() *pe.PinErr {
	if err := s.DB.Close(); err != nil {
		return pe.ErrServiceFailure("failed close Redis client").WithCause(err)
	}
	return nil
}
