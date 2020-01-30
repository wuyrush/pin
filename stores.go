package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Interface of db used in the application.
// This is to retain the flexibility to adopt multiple kinds of db and switching.
type pinStore interface {
	Get(pinID string) (*pin, *pinErr)
	Save(p *pin) *pinErr
	Delete(pinID string) *pinErr
	Close() *pinErr
}

type redisStore struct {
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
	// template to form an unique identifier for individual pin attachment
	memberTmplPinAttachment = `%s.%s`
)

func (s *redisStore) Save(p *pin) *pinErr {
	clog := log.WithField("pinID", p.ID).WithField("pin", p)
	// 1. record the pin for expiry first so that we always have chances to clean it up
	if err := s.markForExpiry(p); err != nil {
		msg := "error recording pin for expiry in Redis"
		clog.WithError(err).Error(msg)
		return errServiceFailure(msg).WithCause(err)
	}
	// 2. record pin data for future access
	if err := s.save(p); err != nil {
		msg := "error saving pin data in Redis"
		clog.WithError(err).Error(msg)
		return errServiceFailure(msg).WithCause(err)
	}
	return nil
}

func (s *redisStore) markForExpiry(p *pin) error {
	expiry := p.CreationTime.Add(p.GoodFor).Unix()
	members := make([]redis.Z, 1+len(p.Attachments))
	members[0] = redis.Z{
		Score:  float64(expiry),
		Member: p.ID,
	}
	cnt := 1
	for _, ref := range p.Attachments {
		members[cnt] = redis.Z{
			Score:  float64(expiry),
			Member: fmt.Sprintf(memberTmplPinAttachment, p.ID, ref),
		}
		cnt++
	}
	if _, err := s.DB.ZAddNX(keyPinExpirySet, members...).Result(); err != nil {
		return err
	}
	return nil
}

// hacks redis keys so that data in the nested object can be store in a flat map
func (s *redisStore) save(p *pin) error {
	clog := log.WithField("pinID", p.ID)
	filesBytes, err := json.Marshal(p.Attachments)
	if err != nil {
		clog.WithError(err).Error("error marshalling pin attachment metadata")
		return err
	}
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
		return err
	}
	if _, err := s.DB.Expire(p.ID, p.GoodFor).Result(); err != nil {
		log.WithError(err).WithField("pinID", p.ID).Error("error setting expiry of existing pin")
		return err
	}
	return nil
}

func (s *redisStore) Get(pinID string) (*pin, *pinErr) {
	clog := log.WithField("pinID", pinID)
	m, err := s.DB.HGetAll(pinID).Result()
	if err != nil {
		msg := "error getting pin data from Redis"
		clog.WithError(err).Error(msg)
		return nil, errServiceFailure(msg).WithCause(err)
	}
	// if Redis had expired the pin the API will return an empty map
	if m == nil || len(m) == 0 {
		return nil, errNotFound(fmt.Sprintf("pin %s not found", pinID))
	}
	p := &pin{
		ID:      pinID,
		OwnerID: m[fieldNameOwnerID],
		Title:   m[fieldNameTitle],
		Note:    m[fieldNameNote],
	}
	mode, err := strconv.Atoi(m[fieldNameMode])
	if err != nil {
		msg := "error unmarshalling access mode"
		clog.WithError(err).Error(msg)
		return nil, errServiceFailure(msg).WithCause(err)
	}
	p.Mode = accessMode(mode)

	vc, err := strconv.ParseInt(m[fieldNameViewCount], 10, 64)
	if err != nil {
		msg := "error unmarshalling view count"
		clog.WithError(err).Error(msg)
		return nil, errServiceFailure(msg).WithCause(err)
	}
	p.ViewCount = uint64(vc)

	var t time.Time
	if err := t.UnmarshalBinary([]byte(m[fieldNameCreationTime])); err != nil {
		msg := "error unmarshalling pin creation time"
		clog.WithError(err).Error(msg)
		return nil, errServiceFailure(msg).WithCause(err)
	}
	p.CreationTime = t

	gf, err := strconv.ParseInt(m[fieldNameGoodFor], 10, 64)
	if err != nil {
		msg := "error unmarshalling good-for period"
		clog.WithError(err).Error(msg)
		return nil, errServiceFailure(msg).WithCause(err)
	}
	p.GoodFor = time.Duration(gf)

	attachments := make(map[string]string)
	if m[fieldNameAttachments] != "" {
		if err := json.Unmarshal([]byte(m[fieldNameAttachments]), &attachments); err != nil {
			msg := "error unmarshalling pin attachment metadata from Redis"
			clog.WithError(err).Error(msg)
			return nil, errServiceFailure(msg).WithCause(err)
		}
	}
	p.Attachments = attachments
	return p, nil
}

func (s *redisStore) Delete(pinID string) *pinErr {
	clog := log.WithField("pinID", pinID)
	if _, err := s.DB.Del(pinID).Result(); err != nil {
		if err != redis.Nil {
			msg := "error deleting pin data from Redis"
			clog.WithError(err).Error(msg)
			return errServiceFailure(msg).WithCause(err)
		}
	}
	return nil
}

func (s *redisStore) Close() *pinErr {
	if err := s.DB.Close(); err != nil {
		return errServiceFailure("failed close Redis client").WithCause(err)
	}
	return nil
}

// fileStore stores files of arbitrary type associated with a given pin
// (note a file is just a byte sequence)
type fileStore interface {
	// Ref returns the reference of file in file storage layer for future persistence and access.
	Ref(pinID, filename string) string
	Save(ref string, r io.ReadCloser) *pinErr
	Get(ref string) (io.ReadCloser, *pinErr)
	Delete(ref string) *pinErr
	Close() *pinErr
}

/*
1. To not leak attachments and exhaust storage capacity, we need to ensure we can still remove the attachments of a given pin even after the pin data is gone
*/

// localFileStore implements fileStore backed by local file system
type localFileStore struct {
}

func (fs *localFileStore) Ref(pinID, filename string) string {
	// TODO: this doesn't scale under high write traffic due to inode exhausation. Essentially local fs storage solution won't scale at all;
	// leveraging third-party services like S3 if pins with attachments are really growing
	return filepath.Join(string(filepath.Separator), "tmp", pinID, filename)
}

func (fs *localFileStore) Save(ref string, r io.ReadCloser) *pinErr {
	pinAttachmentMaxSizeByte := viper.GetInt64(envPinAttachmentSizeMaxByte)
	// 1. prepare file to host data
	errMsg := "error allocating file storage space"
	dir := filepath.Dir(ref)
	if err := os.MkdirAll(dir, os.ModeDir); err != nil {
		return errServiceFailure(errMsg).WithCause(err)
	}
	f, err := os.Create(ref)
	defer f.Close()
	if err != nil {
		return errServiceFailure(errMsg).WithCause(err)
	}
	// 2. pipe data to file
	br := bufio.NewReader(http.MaxBytesReader(nil, r, pinAttachmentMaxSizeByte))
	if _, err := br.WriteTo(f); err != nil {
		if strings.Index(err.Error(), errMsgRequestBodyTooLarge) >= 0 {
			return errBadRequest("pin attachment oversized").WithCause(err)
		}
		return errServiceFailure("error saving pin attachment data").WithCause(err)
	}
	return nil
}

func (fs *localFileStore) Get(ref string) (io.ReadCloser, *pinErr) {
	return nil, nil
}

func (fs *localFileStore) Delete(ref string) *pinErr {
	return nil
}

func (fs *localFileStore) Close() *pinErr {
	return nil
}
