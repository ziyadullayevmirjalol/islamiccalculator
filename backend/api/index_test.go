package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Cold-start smoke test: the function must initialize from pure env vars
// and serve requests. /healthz needs no database round-trip.
func TestHandler_ColdStart(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PASSWORD", "unused")

	rec := httptest.NewRecorder()
	Handler(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("healthz = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	// Second request reuses the initialized router.
	rec = httptest.NewRecorder()
	Handler(rec, httptest.NewRequest(http.MethodGet, "/api/v1/docs/openapi.yaml", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("docs = %d, want 200", rec.Code)
	}
}
