package apperr

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTTPStatus(t *testing.T) {
	cases := []struct {
		name string
		err  *Error
		want int
	}{
		{"validation maps to 400", Validation("bad input", nil), http.StatusBadRequest},
		{"not found maps to 404", NotFound("missing"), http.StatusNotFound},
		{"internal maps to 500", Internal("boom", errors.New("cause")), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.err.HTTPStatus())
		})
	}
}

func TestFrom(t *testing.T) {
	t.Run("passes typed errors through", func(t *testing.T) {
		orig := Validation("bad", map[string]string{"f": "required"})
		assert.Same(t, orig, From(orig))
	})

	t.Run("wraps unknown errors as internal with generic message", func(t *testing.T) {
		e := From(errors.New("pq: password authentication failed"))
		assert.Equal(t, CodeInternal, e.Code)
		assert.Equal(t, "internal server error", e.Message)
		assert.NotContains(t, e.Message, "password")
	})

	t.Run("unwraps through fmt wrapping", func(t *testing.T) {
		orig := NotFound("no such rate")
		wrapped := errors.Join(errors.New("ctx"), orig)
		assert.Equal(t, CodeNotFound, From(wrapped).Code)
	})
}
