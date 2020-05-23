package store

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	se "wuyrush.io/pin/errors"
	md "wuyrush.io/pin/models"
)

func TestCouchPinStore_Create(t *testing.T) {
	fakeDBAddr, fakeUsername, fakePasswd := "http://fake-db:5984", "fakeusername", "fakepasswd"
	fakePinDBName := "fake-pin-db"
	pin := &md.Pin{
		ID:      "0ujsszwN8NRY24YaXiTIE2VWDTS",
		OwnerID: "foo",
	}
	tcs := []struct {
		name       string
		pin        *md.Pin
		rt         *mockTransport
		failed     bool
		expErrCode se.ErrCode
		expErrMsg  string
	}{
		{
			name: "HappyCase",
			pin:  pin,
			rt: func() *mockTransport {
				m := &mockTransport{}
				m.On("RoundTrip", mock.Anything).Run(func(args mock.Arguments) {
					req := args.Get(0).(*http.Request)
					assert.Equal(t, http.MethodPut, req.Method)
					assert.Equal(t, fmt.Sprintf("/%s/%s", fakePinDBName, pin.ID), req.URL.Path)
					assert.Contains(t, fakeDBAddr, req.URL.Hostname())
					uname, passwd, ok := req.BasicAuth()
					assert.True(t, ok)
					assert.Equal(t, fakeUsername, uname)
					assert.Equal(t, fakePasswd, passwd)
				}).Return(
					&http.Response{
						Body:       ioutil.NopCloser(bytes.NewReader([]byte{})),
						StatusCode: http.StatusOK,
					},
					nil,
				)
				return m
			}(),
		},
		{
			name: "NetworkError",
			pin:  pin,
			rt: func() *mockTransport {
				m := &mockTransport{}
				m.On("RoundTrip", mock.Anything).Run(func(args mock.Arguments) {
					req := args.Get(0).(*http.Request)
					assert.Equal(t, http.MethodPut, req.Method)
					assert.Equal(t, fmt.Sprintf("/%s/%s", fakePinDBName, pin.ID), req.URL.Path)
					assert.Contains(t, fakeDBAddr, req.URL.Hostname())
					uname, passwd, ok := req.BasicAuth()
					assert.True(t, ok)
					assert.Equal(t, fakeUsername, uname)
					assert.Equal(t, fakePasswd, passwd)
				}).Return(
					(*http.Response)(nil),
					&net.AddrError{Err: "no internet"},
				)
				return m
			}(),
			failed:     true,
			expErrCode: se.ErrCodeServiceFailure,
			expErrMsg:  "error getting response from DB",
		},
		{
			name: "ErrorReadingResponse",
			pin:  pin,
			rt: func() *mockTransport {
				m := &mockTransport{}
				m.On("RoundTrip", mock.Anything).Run(func(args mock.Arguments) {
					req := args.Get(0).(*http.Request)
					assert.Equal(t, http.MethodPut, req.Method)
					assert.Equal(t, fmt.Sprintf("/%s/%s", fakePinDBName, pin.ID), req.URL.Path)
					assert.Contains(t, fakeDBAddr, req.URL.Hostname())
					uname, passwd, ok := req.BasicAuth()
					assert.True(t, ok)
					assert.Equal(t, fakeUsername, uname)
					assert.Equal(t, fakePasswd, passwd)
				}).Return(
					&http.Response{
						Body:       ioutil.NopCloser(bytes.NewReader([]byte("junk"))),
						StatusCode: http.StatusBadRequest,
					},
					nil,
				)
				return m
			}(),
			failed:     true,
			expErrCode: se.ErrCodeServiceFailure,
			expErrMsg:  "failed to unmarshal CouchDB response body",
		},
		{
			name: "ErrorFromCouchDB",
			pin:  pin,
			rt: func() *mockTransport {
				m := &mockTransport{}
				m.On("RoundTrip", mock.Anything).Run(func(args mock.Arguments) {
					req := args.Get(0).(*http.Request)
					assert.Equal(t, http.MethodPut, req.Method)
					assert.Equal(t, fmt.Sprintf("/%s/%s", fakePinDBName, pin.ID), req.URL.Path)
					assert.Contains(t, fakeDBAddr, req.URL.Hostname())
					uname, passwd, ok := req.BasicAuth()
					assert.True(t, ok)
					assert.Equal(t, fakeUsername, uname)
					assert.Equal(t, fakePasswd, passwd)
				}).Return(
					func() *http.Response {

						r := &http.Response{
							StatusCode: http.StatusInternalServerError,
						}
						couchDBResp := `
{
	"error": "DB nuked",
	"reason": "hacked"
}
					`
						r.Body = ioutil.NopCloser(bytes.NewReader([]byte(couchDBResp)))
						return r
					}(),
					nil,
				)
				return m
			}(),
			failed:     true,
			expErrCode: se.ErrCodeServiceFailure,
			expErrMsg:  "error: DB nuked reason: hacked",
		},
	}
	for _, c := range tcs {
		t.Run(c.name, func(t *testing.T) {
			pst := NewCouchPinStore(&CouchConfig{
				RT:         c.rt,
				PinDBName:  fakePinDBName,
				DBAddr:     fakeDBAddr,
				DBUsername: fakeUsername,
				DBPasswd:   fakePasswd,
			})
			// when
			err := pst.Create(pin)
			c.rt.AssertExpectations(t)
			if c.failed {
				assert.Equal(t, c.expErrCode, err.Code)
				assert.Contains(t, err.Error(), c.expErrMsg)
			}
		})
	}
}

type mockTransport struct {
	http.RoundTripper
	mock.Mock
}

func (m *mockTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	args := m.Called(r)
	return args.Get(0).(*http.Response), args.Error(1)
}
