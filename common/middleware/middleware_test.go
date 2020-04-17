package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	hr "github.com/julienschmidt/httprouter"
	"github.com/magiconair/properties/assert"
)

func TestPanicRecover(t *testing.T) {
	wrec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/fake", nil)
	prm := hr.Param{Key: "foo", Value: "bar"}
	cnt := 0
	touch := func() { cnt++ }
	h := func(w http.ResponseWriter, r *http.Request, p hr.Params) {
		touch()
		// params are passed through as expected
		assert.Equal(t, wrec, w, "unexpected response writer")
		assert.Equal(t, req, r, "unexpected request value")
		assert.Equal(t, hr.Params{prm}, p, "unexpected request value")
		panic("boom!")
	}
	wrapped := Chain(h, PanicRecoverer())

	wrapped(wrec, req, hr.Params{prm})
	assert.Equal(t, 1, cnt, "underlyig handler not called by middleware")
}
