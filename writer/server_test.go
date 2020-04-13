package main

import (
	"html/template"
	"io"
	"strings"
	"testing"

	"net/http"
	"net/http/httptest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	se "wuyrush.io/pin/errors"
	md "wuyrush.io/pin/models"
)

const (
	testTrapName = "faketrap"
)

func TestHandleTaskGetCreatePinPage(t *testing.T) {
	tcs := []struct {
		name               string
		expectedStatusCode int
	}{
		{
			name:               "HappyCase",
			expectedStatusCode: http.StatusOK,
		},
	}
	fakeTmpl, err := template.New("fakeTmpl").Parse(`{{.Private}}`)
	assert.Nil(t, err, "parsing fake template should have succeeded")
	for _, c := range tcs {
		t.Run(c.name, func(t *testing.T) {
			wrec, r := httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/create", nil)
			wrt := &writer{}
			getPage := wrt.HandleTaskGetCreatePinPage(fakeTmpl)
			getPage(wrec, r, nil)

			assert.Equal(t, c.expectedStatusCode, wrec.Code, "unexpected response status code")
		})
	}
}

func TestHandleTaskCreatePin(t *testing.T) {
	type formView struct {
		Trap, Title, Private, ReadOnlyOnce, Body, GoodFor string
	}
	goodFormView := func() formView {
		return formView{
			Title:        "fakeTitle",
			Private:      "true",
			ReadOnlyOnce: "true",
			Body:         "fakeBody",
			GoodFor:      "1h",
		}
	}
	tcs := []struct {
		name         string
		reqBody      io.Reader
		expectedCode int
	}{
		{
			name:         "HappyCaseWithoutFiles",
			reqBody:      genCreatePinReqBody(goodFormView()),
			expectedCode: http.StatusOK,
		},
		//		{
		//			name: "HappyCaseWithFiles",
		//		},
		//		{
		//			name: "EmptyTitle",
		//		},
		{
			name: "SpamAttempt",
			reqBody: func() io.Reader {
				v := goodFormView()
				v.Trap = "y"
				return genCreatePinReqBody(v)
			}(),
			expectedCode: http.StatusForbidden,
		},
		{
			name: "OversizedTitle",
			reqBody: func() io.Reader {
				v := goodFormView()
				v.Title = strings.Repeat("omyverylongtitle", 1<<5) // 1<<9 bytes in total
				return genCreatePinReqBody(v)
			}(),
			expectedCode: http.StatusBadRequest,
		},
		{
			name: "EmptyTitle",
			reqBody: func() io.Reader {
				v := goodFormView()
				v.Title = ""
				return genCreatePinReqBody(v)
			}(),
			expectedCode: http.StatusOK,
		},
		{
			name: "OversizedBody",
			reqBody: func() io.Reader {
				v := goodFormView()
				v.Body = strings.Repeat("ohmyverylongbody", 1<<15) // 1<<19 bytes in total
				return genCreatePinReqBody(v)
			}(),
			expectedCode: http.StatusBadRequest,
		},
		{
			name: "EmptyBody",
			reqBody: func() io.Reader {
				v := goodFormView()
				v.Body = ""
				return genCreatePinReqBody(v)
			}(),
			expectedCode: http.StatusBadRequest,
		},
		{
			name: "InvalidAccessMode",
			reqBody: func() io.Reader {
				v := goodFormView()
				v.Private = "junk"
				return genCreatePinReqBody(v)
			}(),
			expectedCode: http.StatusBadRequest,
		},
		{
			name: "InvalidReadOnlyOnce",
			reqBody: func() io.Reader {
				v := goodFormView()
				v.ReadOnlyOnce = "junk"
				return genCreatePinReqBody(v)
			}(),
			expectedCode: http.StatusBadRequest,
		},
		{
			name: "InvalidGoodFor",
			reqBody: func() io.Reader {
				v := goodFormView()
				v.GoodFor = "junk"
				return genCreatePinReqBody(v)
			}(),
			expectedCode: http.StatusBadRequest,
		},
		{
			name: "GoodForTooShort",
			reqBody: func() io.Reader {
				v := goodFormView()
				v.GoodFor = "30s"
				return genCreatePinReqBody(v)
			}(),
			expectedCode: http.StatusBadRequest,
		},
		{
			name: "GoodForTooLong",
			reqBody: func() io.Reader {
				v := goodFormView()
				v.GoodFor = "2h"
				return genCreatePinReqBody(v)
			}(),
			expectedCode: http.StatusBadRequest,
		},
		//{
		//	name: "PinDaoError",
		//},
	}
	fakeTmpl, err := template.New("fakeTmpl").Parse(`
		{{.Err}}	
		{{.Title}}	
		{{.Body}}	
	`)
	assert.Nil(t, err, "parsing fake template should have succeeded")
	for _, c := range tcs {
		t.Run(c.name, func(t *testing.T) {
			// given
			mockPinDao := &MockPinDAO{}
			mockPinDao.On("Create", mock.AnythingOfType("*models.Pin")).Return((*se.Err)(nil))
			wrec, r := httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/create", c.reqBody)
			r.Header.Add("Content-Type", "multipart/form-data;boundary=\"test\"")
			wrt := &writer{PinDAO: mockPinDao}
			createPin := wrt.HandleTaskCreatePin(fakeTmpl, testTrapName)
			// when
			createPin(wrec, r, nil)
			// then
			assert.Equal(t, c.expectedCode, wrec.Code, "unexpected response status code")
			// TODO: more verification on response data
		})
	}
}

func TestHandleTaskChangePinAccessMode(t *testing.T) {

}

func TestHandleTaskDeletePin(t *testing.T) {

}

func TestHandleAuthRegister(t *testing.T) {

}

func TestHandleAuthLogin(t *testing.T) {

}

func TestHandleAuthLogout(t *testing.T) {

}

func TestHandleAuthUpdateUserProfile(t *testing.T) {

}

// mocks
type MockPinDAO struct{ mock.Mock }

func (m *MockPinDAO) Create(p *md.Pin) *se.Err {
	return m.Called(p).Get(0).(*se.Err)
}

// utils

func genCreatePinReqBody(data interface{}) io.Reader {
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Methods/POST
	tmpl := template.Must(template.New("reqBodyTmpl").Parse(
		`
The preamble of multipart request body. This should be ignored.
--test
Content-Disposition: form-data; name="faketrap" 

{{.Trap}}
--test
Content-Disposition: form-data; name="title" 

{{.Title}}
--test
Content-Disposition: form-data; name="private" 

{{.Private}}
--test
Content-Disposition: form-data; name="read-only-once" 

{{.ReadOnlyOnce}}
--test
Content-Disposition: form-data; name="body" 

{{.Body}}
--test
Content-Disposition: form-data; name="good-for" 

{{.GoodFor}}
--test--
The epilogue. This should be ignored
		`))
	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		panic(err)
	}
	return strings.NewReader(b.String())
}
