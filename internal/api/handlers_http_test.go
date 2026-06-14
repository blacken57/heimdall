package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleIndex_OK(t *testing.T) {
	chdirToRepoRoot(t)
	srv := newTestServer(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<html") {
		t.Error("expected a full HTML page (containing <html>)")
	}
	if !strings.Contains(body, "Test") {
		t.Error("expected the seeded service name 'Test' in the page")
	}
}

func TestHandleIndex_NotFoundForUnknownPath(t *testing.T) {
	chdirToRepoRoot(t)
	srv := newTestServer(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestHandlePartialServices_ReturnsFragment(t *testing.T) {
	chdirToRepoRoot(t)
	srv := newTestServer(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/partials/services", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	// A partial must be a fragment, not a full page.
	if strings.Contains(body, "<html") || strings.Contains(body, "<!DOCTYPE") {
		t.Error("partial should be a fragment, not a full HTML document")
	}
	if !strings.Contains(body, "Test") {
		t.Error("expected the seeded service name 'Test' in the fragment")
	}
}

func TestHandleDayDetail_InvalidServiceID(t *testing.T) {
	srv := newTestServer(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/partials/day-detail?service_id=abc&date=2026-06-14", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleDayDetail_InvalidDate(t *testing.T) {
	srv := newTestServer(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/partials/day-detail?service_id=1&date=bad", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleDayDetail_OK(t *testing.T) {
	chdirToRepoRoot(t)
	srv := newTestServer(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/partials/day-detail?service_id=1&date=2026-06-14", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "hourly breakdown") {
		t.Error("expected the hourly-breakdown title in the day-detail fragment")
	}
	// The modal must nest the popup inside the backdrop (the fix from earlier).
	if !strings.Contains(body, "day-detail-backdrop") || !strings.Contains(body, "day-detail-popup") {
		t.Error("expected backdrop and popup elements in the day-detail fragment")
	}
}
