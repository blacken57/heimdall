package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/blacken57/heimdall/internal/config"
	"github.com/blacken57/heimdall/internal/db"
)

// newTestChecker builds a Checker backed by an in-memory DB with a single
// service already registered. It returns the checker and the service's id.
func newTestChecker(t *testing.T, svc config.Service) (*Checker, *db.DB, int64) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	id, err := d.UpsertService(svc.Name, svc.URL)
	if err != nil {
		t.Fatalf("upsert service: %v", err)
	}

	cfg := &config.Config{
		Services:     []config.Service{svc},
		PollInterval: 60,
		HTTPTimeout:  10,
	}
	c := New(cfg, d, map[string]int64{svc.Name: id})
	return c, d, id
}

// lastCheck reads the most recent check row for a service.
func lastCheck(t *testing.T, d *db.DB, id int64) (statusCode, responseMs int, isUp bool, errMsg string) {
	t.Helper()
	summaries, err := d.GetAllServiceSummaries()
	if err != nil {
		t.Fatalf("GetAllServiceSummaries: %v", err)
	}
	for _, s := range summaries {
		if s.ID == id {
			return s.StatusCode, int(s.AvgResponseMs), s.IsUp, s.ErrorMessage
		}
	}
	t.Fatalf("no summary for service id %d", id)
	return 0, 0, false, ""
}

func TestPoll_Up(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := config.Service{Name: "Up", URL: srv.URL}
	c, d, id := newTestChecker(t, svc)

	c.poll(svc, id)

	code, _, isUp, errMsg := lastCheck(t, d, id)
	if !isUp {
		t.Error("expected isUp=true for 200 response")
	}
	if code != http.StatusOK {
		t.Errorf("status code: got %d, want 200", code)
	}
	if errMsg != "" {
		t.Errorf("expected empty error message, got %q", errMsg)
	}
}

func TestPoll_Down5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	svc := config.Service{Name: "Down", URL: srv.URL}
	c, d, id := newTestChecker(t, svc)

	c.poll(svc, id)

	code, _, isUp, _ := lastCheck(t, d, id)
	if isUp {
		t.Error("expected isUp=false for 500 response")
	}
	if code != http.StatusInternalServerError {
		t.Errorf("status code: got %d, want 500", code)
	}
}

func TestPoll_Down4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := config.Service{Name: "NotFound", URL: srv.URL}
	c, d, id := newTestChecker(t, svc)

	c.poll(svc, id)

	code, _, isUp, _ := lastCheck(t, d, id)
	if isUp {
		t.Error("expected isUp=false for 404 response")
	}
	if code != http.StatusNotFound {
		t.Errorf("status code: got %d, want 404", code)
	}
}

// A 3xx response should be treated as UP: the client is configured with
// http.ErrUseLastResponse so it returns the redirect itself rather than
// following it, and 3xx is within the [200,400) "up" range.
func TestPoll_RedirectIsUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.com/elsewhere", http.StatusFound)
	}))
	defer srv.Close()

	svc := config.Service{Name: "Redirect", URL: srv.URL}
	c, d, id := newTestChecker(t, svc)

	c.poll(svc, id)

	code, _, isUp, _ := lastCheck(t, d, id)
	if !isUp {
		t.Error("expected isUp=true for 302 response (treated as up)")
	}
	if code != http.StatusFound {
		t.Errorf("status code: got %d, want 302", code)
	}
}

func TestPoll_ConnectionError(t *testing.T) {
	// Point at a server we immediately close so the connection is refused.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	svc := config.Service{Name: "Unreachable", URL: url}
	c, d, id := newTestChecker(t, svc)

	c.poll(svc, id)

	code, _, isUp, errMsg := lastCheck(t, d, id)
	if isUp {
		t.Error("expected isUp=false on connection error")
	}
	if code != 0 {
		t.Errorf("status code: got %d, want 0 on connection error", code)
	}
	if errMsg == "" {
		t.Error("expected a non-empty error message on connection error")
	}
}

func TestPoll_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := config.Service{Name: "Slow", URL: srv.URL}
	c, d, id := newTestChecker(t, svc)
	// Override the client with a sub-second timeout to keep the test fast.
	c.client = &http.Client{Timeout: 50 * time.Millisecond}

	c.poll(svc, id)

	_, _, isUp, errMsg := lastCheck(t, d, id)
	if isUp {
		t.Error("expected isUp=false when the request times out")
	}
	if errMsg == "" {
		t.Error("expected a non-empty error message on timeout")
	}
}

// runService should perform an immediate first poll, then stop promptly when
// the context is cancelled (rather than waiting a full poll interval).
func TestRunService_ImmediatePollThenShutdown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := config.Service{Name: "Loop", URL: srv.URL}
	c, d, _ := newTestChecker(t, svc)
	// A long interval guarantees the ticker never fires during the test, so
	// any recorded check must be the immediate first poll.
	c.cfg.PollInterval = 3600

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		c.runService(ctx, svc)
		close(done)
	}()

	// Give the immediate poll time to land, then cancel.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runService did not return after context cancellation")
	}

	summaries, err := d.GetAllServiceSummaries()
	if err != nil {
		t.Fatalf("GetAllServiceSummaries: %v", err)
	}
	if summaries[0].TotalChecks != 1 {
		t.Errorf("expected exactly 1 check from immediate poll, got %d", summaries[0].TotalChecks)
	}
}

// An unknown service (not present in the serviceIDs map) should be skipped
// without panicking or recording any check.
func TestRunService_UnknownService(t *testing.T) {
	svc := config.Service{Name: "Ghost", URL: "https://example.com"}
	c, d, _ := newTestChecker(t, config.Service{Name: "Real", URL: "https://example.com"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled; should return immediately regardless

	// svc.Name "Ghost" is not in the serviceIDs map → early return.
	c.runService(ctx, svc)

	summaries, err := d.GetAllServiceSummaries()
	if err != nil {
		t.Fatalf("GetAllServiceSummaries: %v", err)
	}
	for _, s := range summaries {
		if s.TotalChecks != 0 {
			t.Errorf("expected no checks for unknown service, got %d for %q", s.TotalChecks, s.Name)
		}
	}
}
