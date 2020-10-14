package health

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/assert"
)

func TestHealthHandler(t *testing.T) {
	failing := HttpHandler(map[string]Checker{
		"mock": healthCheckFunc(func() error { return errors.New("err") }),
	})
	ok := HttpHandler(map[string]Checker{
		"mock": healthCheckFunc(func() error { return nil }),
	})
	var httpTests = []struct {
		wantHeader int
		handler    http.Handler
	}{
		{200, ok},
		{500, failing},
	}
	for _, tt := range httpTests {
		t.Run("", func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/healthz", nil)
			tt.handler.ServeHTTP(rr, req)
			assert.Equal(t, rr.Code, tt.wantHeader)
		})
	}
}

type healthCheckFunc func() error

func (fn healthCheckFunc) HealthCheck() error {
	return fn()
}
