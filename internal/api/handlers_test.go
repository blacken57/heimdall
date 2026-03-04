package api

import (
	"testing"
	"time"

	"github.com/blacken57/heimdall/internal/db"
)

func noDataHistory() []db.DayBucket {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	buckets := make([]db.DayBucket, 30)
	for i := 0; i < 30; i++ {
		buckets[i] = db.DayBucket{Date: today.AddDate(0, 0, i-29)}
	}
	return buckets
}

func baseSummary() db.ServiceSummary {
	return db.ServiceSummary{
		ID:            1,
		Name:          "Test",
		URL:           "https://example.com",
		IsUp:          true,
		UptimePct:     100.0,
		AvgResponseMs: 100.0,
		LastChecked:   time.Now().UTC(),
		TotalChecks:   10,
		History:       noDataHistory(),
	}
}

func TestToView_Pending(t *testing.T) {
	s := baseSummary()
	s.LastChecked = time.Time{} // zero time = no checks yet

	v := toView(s)
	if v.StatusBadge != "PENDING" {
		t.Errorf("StatusBadge: got %q, want %q", v.StatusBadge, "PENDING")
	}
	if v.UptimePct != "—" {
		t.Errorf("UptimePct: got %q, want %q", v.UptimePct, "—")
	}
	if v.LastChecked != "never" {
		t.Errorf("LastChecked: got %q, want %q", v.LastChecked, "never")
	}
}

func TestToView_Up(t *testing.T) {
	s := baseSummary()
	s.IsUp = true

	v := toView(s)
	if v.StatusBadge != "UP" {
		t.Errorf("StatusBadge: got %q, want %q", v.StatusBadge, "UP")
	}
	if !v.HasData {
		t.Error("HasData: expected true for non-zero LastChecked")
	}
}

func TestToView_Down(t *testing.T) {
	s := baseSummary()
	s.IsUp = false

	v := toView(s)
	if v.StatusBadge != "DOWN" {
		t.Errorf("StatusBadge: got %q, want %q", v.StatusBadge, "DOWN")
	}
}

func TestToView_UptimeFormat(t *testing.T) {
	s := baseSummary()
	s.UptimePct = 99.9
	v := toView(s)
	if v.UptimePct != "99.9%" {
		t.Errorf("UptimePct 99.9: got %q, want %q", v.UptimePct, "99.9%")
	}

	s.UptimePct = 0.0
	v = toView(s)
	if v.UptimePct != "0.0%" {
		t.Errorf("UptimePct 0.0: got %q, want %q", v.UptimePct, "0.0%")
	}
}

func TestToView_AvgResponseFormat(t *testing.T) {
	s := baseSummary()
	s.AvgResponseMs = 123.7
	v := toView(s)
	if v.AvgResponseMs != "124 ms" {
		t.Errorf("AvgResponseMs: got %q, want %q", v.AvgResponseMs, "124 ms")
	}
}

func TestToView_LastChecked_Seconds(t *testing.T) {
	s := baseSummary()
	s.LastChecked = time.Now().UTC().Add(-30 * time.Second)
	v := toView(s)
	if v.LastChecked != "30s ago" {
		t.Errorf("LastChecked: got %q, want %q", v.LastChecked, "30s ago")
	}
}

func TestToView_LastChecked_Minutes(t *testing.T) {
	s := baseSummary()
	s.LastChecked = time.Now().UTC().Add(-5 * time.Minute)
	v := toView(s)
	if v.LastChecked != "5m ago" {
		t.Errorf("LastChecked: got %q, want %q", v.LastChecked, "5m ago")
	}
}

func TestToView_LastChecked_Older(t *testing.T) {
	s := baseSummary()
	t2h := time.Now().UTC().Add(-2 * time.Hour)
	s.LastChecked = t2h
	v := toView(s)
	expected := t2h.Format("Jan 2 15:04")
	if v.LastChecked != expected {
		t.Errorf("LastChecked: got %q, want %q", v.LastChecked, expected)
	}
}

func TestToView_HistoryAllNoData(t *testing.T) {
	s := baseSummary()
	v := toView(s)
	if len(v.History) != 30 {
		t.Fatalf("expected 30 history segments, got %d", len(v.History))
	}
	for i, seg := range v.History {
		if seg.Color != "no-data" {
			t.Errorf("segment[%d]: Color=%q, want %q", i, seg.Color, "no-data")
		}
	}
}

func TestToView_HistoryUp(t *testing.T) {
	s := baseSummary()
	s.History[0] = db.DayBucket{
		Date:        s.History[0].Date,
		HasData:     true,
		TotalChecks: 10,
		UpChecks:    10, // 100%
	}
	v := toView(s)
	if v.History[0].Color != "up" {
		t.Errorf("History[0].Color: got %q, want %q", v.History[0].Color, "up")
	}
}

func TestToView_HistoryDown(t *testing.T) {
	s := baseSummary()
	s.History[0] = db.DayBucket{
		Date:        s.History[0].Date,
		HasData:     true,
		TotalChecks: 10,
		UpChecks:    5, // 50%
	}
	v := toView(s)
	if v.History[0].Color != "down" {
		t.Errorf("History[0].Color: got %q, want %q", v.History[0].Color, "down")
	}
}

func TestToView_HistoryBoundary(t *testing.T) {
	s := baseSummary()

	// 9/10 = 90% → "up"
	s.History[0] = db.DayBucket{
		Date:        s.History[0].Date,
		HasData:     true,
		TotalChecks: 10,
		UpChecks:    9,
	}
	v := toView(s)
	if v.History[0].Color != "up" {
		t.Errorf("90%%: got %q, want %q", v.History[0].Color, "up")
	}

	// 8/10 = 80% → "down"
	s.History[0] = db.DayBucket{
		Date:        s.History[0].Date,
		HasData:     true,
		TotalChecks: 10,
		UpChecks:    8,
	}
	v = toView(s)
	if v.History[0].Color != "down" {
		t.Errorf("80%%: got %q, want %q", v.History[0].Color, "down")
	}
}

func TestToView_TotalChecks(t *testing.T) {
	s := baseSummary()
	s.TotalChecks = 42
	v := toView(s)
	if v.TotalChecks != "42 checks" {
		t.Errorf("TotalChecks: got %q, want %q", v.TotalChecks, "42 checks")
	}
}
