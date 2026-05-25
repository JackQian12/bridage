package httpserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nuts/bridage/internal/admin"
	"github.com/nuts/bridage/internal/httpserver"
	"github.com/nuts/bridage/internal/publicapi"
	"go.uber.org/zap"
)

// buildTestRouter creates a Gin engine wired with real middleware but nil
// DB stores. Tests must only hit routes that do NOT perform any DB access.
func buildTestRouter(t *testing.T) http.Handler {
	t.Helper()
	log := zap.NewNop()

	// admin.Handler: nil stores are fine as long as routes that use them
	// are not exercised. ValidateJWT uses only jwtSecret.
	adm := admin.NewHandler(
		nil, nil, nil, nil, nil, nil,
		"test-master-key", "test-jwt-secret",
		time.Hour, log,
	)

	// publicapi.Handler: nil stores / relay are fine for routes not exercised.
	pub := publicapi.NewHandler(nil, nil, nil)

	return httpserver.NewRouter(nil, pub, adm, []string{"*"}, log)
}

// ─── /health ─────────────────────────────────────────────────────────────────

func TestHealth_OK(t *testing.T) {
	router := buildTestRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`expected {"status":"ok"}, got %v`, body)
	}
}

func TestHealth_RequestIDHeader(t *testing.T) {
	router := buildTestRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Request-Id", "test-req-id-123")
	router.ServeHTTP(w, req)

	if w.Header().Get("X-Request-Id") != "test-req-id-123" {
		t.Errorf("X-Request-Id not echoed back, got: %q", w.Header().Get("X-Request-Id"))
	}
}

func TestHealth_AutoRequestID(t *testing.T) {
	router := buildTestRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(w, req)

	if w.Header().Get("X-Request-Id") == "" {
		t.Error("expected auto-generated X-Request-Id to be set in response")
	}
}

// ─── JWT middleware ───────────────────────────────────────────────────────────

func TestAdminRoutes_MissingToken(t *testing.T) {
	router := buildTestRouter(t)
	// Any JWT-protected admin route
	for _, path := range []string{
		"/admin/providers",
		"/admin/providers/00000000-0000-0000-0000-000000000001",
		"/admin/models",
		"/admin/models/00000000-0000-0000-0000-000000000001",
		"/admin/keys",
		"/admin/keys/00000000-0000-0000-0000-000000000001",
		"/admin/presets",
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("GET %s: expected 401, got %d", path, w.Code)
		}
	}
}

func TestAdminRoutes_MalformedToken(t *testing.T) {
	router := buildTestRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/providers", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.jwt")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid JWT, got %d", w.Code)
	}
}

// ─── Key auth middleware ──────────────────────────────────────────────────────

func TestV1Routes_MissingKey(t *testing.T) {
	router := buildTestRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing key, got %d", w.Code)
	}
}

func TestV1Routes_WrongKeyPrefix(t *testing.T) {
	router := buildTestRouter(t)
	// A key that doesn't start with brg_ should be rejected before any DB lookup.
	for _, badKey := range []string{"sk-1234567890", "BEARER sk-abc", "apikey-xyz"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		req.Header.Set("Authorization", "Bearer "+badKey)
		router.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("key %q: expected 401, got %d", badKey, w.Code)
		}
	}
}

// ─── CORS ─────────────────────────────────────────────────────────────────────

func TestCORS_WildcardOrigin(t *testing.T) {
	router := buildTestRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	router.ServeHTTP(w, req)

	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin == "" {
		t.Error("expected Access-Control-Allow-Origin header to be set")
	}
}

// ─── 404 ──────────────────────────────────────────────────────────────────────

func TestNotFound(t *testing.T) {
	router := buildTestRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nonexistent-route", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
