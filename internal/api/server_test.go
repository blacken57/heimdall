package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/blacken57/heimdall/internal/config"
	"github.com/blacken57/heimdall/internal/db"
)

// chdirToRepoRoot switches the working directory to the repo root so that the
// handlers' relative template paths ("web/templates/...") resolve. The original
// directory is restored on cleanup. Tests using this must not run in parallel.
func chdirToRepoRoot(t *testing.T) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// internal/api → repo root is two levels up.
	if err := os.Chdir("../.."); err != nil {
		t.Fatalf("chdir to repo root: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}

// newTestServer builds a Server backed by an in-memory DB seeded with one
// service that has a single successful check.
func newTestServer(t *testing.T, cfg *config.Config) *Server {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	id, err := d.UpsertService("Test", "https://example.com")
	if err != nil {
		t.Fatalf("upsert service: %v", err)
	}
	if err := d.InsertCheck(id, 200, 120, true, ""); err != nil {
		t.Fatalf("insert check: %v", err)
	}

	if cfg == nil {
		cfg = &config.Config{}
	}
	return New(cfg, d)
}

func TestHealth_OK(t *testing.T) {
	srv := newTestServer(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != "ok" {
		t.Errorf("body: got %q, want %q", body, "ok")
	}
}

func TestHealth_OpenEvenWithAuth(t *testing.T) {
	cfg := &config.Config{HeimdallUser: "user", HeimdallPassword: "pass"}
	srv := newTestServer(t, cfg)

	// No credentials supplied.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("/health should be open without auth: got %d, want 200", rec.Code)
	}
}

func TestBasicAuth_MissingCredentials(t *testing.T) {
	cfg := &config.Config{HeimdallUser: "user", HeimdallPassword: "pass"}
	srv := newTestServer(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
	if h := rec.Header().Get("WWW-Authenticate"); !strings.Contains(h, "Basic") {
		t.Errorf("WWW-Authenticate header: got %q, want it to contain %q", h, "Basic")
	}
}

func TestBasicAuth_WrongCredentials(t *testing.T) {
	cfg := &config.Config{HeimdallUser: "user", HeimdallPassword: "pass"}
	srv := newTestServer(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("user", "wrong")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestBasicAuth_CorrectCredentials(t *testing.T) {
	chdirToRepoRoot(t)
	cfg := &config.Config{HeimdallUser: "user", HeimdallPassword: "pass"}
	srv := newTestServer(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("user", "pass")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestNoAuth_WhenDisabled(t *testing.T) {
	chdirToRepoRoot(t)
	srv := newTestServer(t, nil) // no user/password → auth disabled

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200 (auth disabled)", rec.Code)
	}
}
