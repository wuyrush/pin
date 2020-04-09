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

func TestHandleTaskCreatePin(t *testing.T) {
	type formView struct {
		Title, Private, ReadOnlyOnce, Body, ExpireAfter string
	}
	goodFormView := func() formView {
		return formView{
			Title:        "fakeTitle",
			Private:      "false",
			ReadOnlyOnce: "false",
			Body:         "fakeBody",
			ExpireAfter:  "1h",
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
		//		{
		//			name: "SpamAttempt",
		//		},
		//		{
		//			name: "OversizedTitle",
		//		},
		//		{
		//			name: "OversizedBody",
		//		},
		//		{
		//			name: "EmptyBody",
		//		},
		//		{
		//			name: "InvalidAccessMode",
		//		},
		//		{
		//			name: "InvalidReadOnlyOnce",
		//		},
		//		{
		//			name: "InvalidExpiry",
		//		},
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
			// TODO: more verification
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
Content-Disposition: form-data; name="expire-after" 

{{.ExpireAfter}}
--test--
The epilogue. This should be ignored
		`))
	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		panic(err)
	}
	return strings.NewReader(b.String())
}
