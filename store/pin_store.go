package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	se "wuyrush.io/pin/errors"
	md "wuyrush.io/pin/models"
)

// PinStore provides mechanism to manage individual pin
type PinStore interface {
	Create(pin *md.Pin) *se.Err
	Close() *se.Err
}

// CouchPinStore implements PinStore with CouchDB
type CouchPinStore struct {
	C                    *http.Client
	dbAddr               string
	pinDBName            string
	pinMetadataDBName    string
	dbUsername, dbPasswd string
}

type CouchConfig struct {
	DBAddr               string
	PinDBName            string
	PinMetadataDBName    string
	DBUsername, DBPasswd string
	RT                   http.RoundTripper
	// fields below are optional
	RequestTimeout time.Duration
}

func NewCouchPinStore(cfg *CouchConfig) *CouchPinStore {
	c := &http.Client{
		Transport: cfg.RT,
		Timeout:   cfg.RequestTimeout,
	}
	return &CouchPinStore{
		C:                 c,
		pinDBName:         cfg.PinDBName,
		pinMetadataDBName: cfg.PinMetadataDBName,
		dbAddr:            cfg.DBAddr,
		dbUsername:        cfg.DBUsername,
		dbPasswd:          cfg.DBPasswd,
	}
}

func (s *CouchPinStore) Create(pin *md.Pin) *se.Err {
	clog := log.WithFields(log.Fields{"pinID": pin.ID, "pinExpiry": pin.CreationTime.Add(pin.GoodFor)})
	// marshal pin into json
	pb, err := json.Marshal(pin)
	if err != nil {
		return se.NewServiceFailure("error marshalling pin data").WithCause(err)
	}
	// call CouchDB to save pin json via http
	url := fmt.Sprintf("%s/%s/%s", s.dbAddr, s.pinDBName, pin.ID)
	// TODO: switch to TLS for better security. Doing this means the underlying HTTP client setup needs to reduce the overhead of
	// frequent TLS handshake between app client and DB backend
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(pb))
	if err != nil {
		return se.NewServiceFailure("error creating request to DB").WithCause(err)
	}
	req.SetBasicAuth(s.dbUsername, s.dbPasswd)
	resp, err := s.C.Do(req)
	// handle failures on http roudtrip and non-2XX responses if any
	if err != nil {
		clog.WithError(err).Error("error getting response from CouchDB")
		return se.NewServiceFailure("error getting response from DB when saving pin").WithCause(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		// TODO: discern client and sever error
		dberr := toCouchDBErr(resp.Body)
		clog.WithError(dberr).Error("failed saving pin to CouchDB")
		return se.NewServiceFailure("failed to save pin").WithCause(dberr)
	}
	return nil
}

func (s *CouchPinStore) Close() *se.Err {
	// release the connections held by C
	s.C.CloseIdleConnections()
	return nil
}

// https://docs.couchdb.org/en/stable/json-structure.html#couchdb-error-status
type CouchDBErr struct {
	DocID  string `json:"id,omitempty"`
	Msg    string `json:"error,omitempty"`
	Reason string `json:"reason,omitempty"`
}

func (e *CouchDBErr) Error() string {
	var b strings.Builder
	if e.Msg != "" {
		b.WriteString("error: ")
		b.WriteString(e.Msg)
	}
	if e.Reason != "" {
		b.WriteString(" reason: ")
		b.WriteString(e.Reason)
	}
	if e.DocID != "" {
		b.WriteString(" docID: ")
		b.WriteString(e.DocID)
	}
	return b.String()
}

func toCouchDBErr(r io.Reader) *CouchDBErr {
	e := &CouchDBErr{}
	err := unmarshalJSON(r, e)
	if err != nil {
		e.Msg = "failed to unmarshal CouchDB response body"
		e.Reason = err.Error()
	}
	return e
}

// helper to unmarshal stream data from r into value pointed by ptr
func unmarshalJSON(r io.Reader, ptr interface{}) error {
	d := json.NewDecoder(r)
	return d.Decode(ptr)
}
