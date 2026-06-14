package db

import (
	"testing"
	"time"
)

// insertCheckAt inserts a check with an explicit checked_at timestamp.
func insertCheckAt(t *testing.T, d *DB, svcID int64, isUp bool, ts time.Time) {
	t.Helper()
	_, err := d.conn.Exec(
		`INSERT INTO checks (service_id, checked_at, status_code, response_ms, is_up) VALUES (?, ?, ?, ?, ?)`,
		svcID, ts.UTC(), 200, 100, isUp,
	)
	if err != nil {
		t.Fatalf("insertCheckAt: %v", err)
	}
}

func TestGetHourlyHistory_NoData(t *testing.T) {
	d := newTestDB(t)
	id, err := d.UpsertService("Alpha", "https://alpha.example.com")
	if err != nil {
		t.Fatalf("UpsertService: %v", err)
	}

	hours, err := d.GetHourlyHistory(id, "2026-06-14")
	if err != nil {
		t.Fatalf("GetHourlyHistory: %v", err)
	}
	if len(hours) != 24 {
		t.Fatalf("expected 24 hour buckets, got %d", len(hours))
	}
	for i, b := range hours {
		if b.Hour != i {
			t.Errorf("bucket[%d].Hour = %d, want %d", i, b.Hour, i)
		}
		if b.HasData {
			t.Errorf("bucket[%d]: expected HasData=false", i)
		}
	}
}

func TestGetHourlyHistory_Buckets(t *testing.T) {
	d := newTestDB(t)
	id, err := d.UpsertService("Alpha", "https://alpha.example.com")
	if err != nil {
		t.Fatalf("UpsertService: %v", err)
	}

	day := "2026-06-14"
	base := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)

	// Hour 10: two checks (one up, one down).
	insertCheckAt(t, d, id, true, base.Add(10*time.Hour))
	insertCheckAt(t, d, id, false, base.Add(10*time.Hour+30*time.Minute))
	// Hour 11: one check (up).
	insertCheckAt(t, d, id, true, base.Add(11*time.Hour))
	// A check on a different day should be ignored.
	insertCheckAt(t, d, id, true, base.AddDate(0, 0, 1).Add(10*time.Hour))

	hours, err := d.GetHourlyHistory(id, day)
	if err != nil {
		t.Fatalf("GetHourlyHistory: %v", err)
	}

	if !hours[10].HasData {
		t.Fatal("hour 10: expected HasData=true")
	}
	if hours[10].TotalChecks != 2 {
		t.Errorf("hour 10 TotalChecks: got %d, want 2", hours[10].TotalChecks)
	}
	if hours[10].UpChecks != 1 {
		t.Errorf("hour 10 UpChecks: got %d, want 1", hours[10].UpChecks)
	}

	if !hours[11].HasData {
		t.Fatal("hour 11: expected HasData=true")
	}
	if hours[11].TotalChecks != 1 {
		t.Errorf("hour 11 TotalChecks: got %d, want 1", hours[11].TotalChecks)
	}

	// Every other hour should be empty.
	for i, b := range hours {
		if i == 10 || i == 11 {
			continue
		}
		if b.HasData {
			t.Errorf("hour %d: expected HasData=false, got data (%d checks)", i, b.TotalChecks)
		}
	}
}

func TestInsertCheck_WithError(t *testing.T) {
	d := newTestDB(t)
	id, err := d.UpsertService("Alpha", "https://alpha.example.com")
	if err != nil {
		t.Fatalf("UpsertService: %v", err)
	}

	if err := d.InsertCheck(id, 0, 0, false, "dial tcp: connection refused"); err != nil {
		t.Fatalf("InsertCheck: %v", err)
	}

	summaries, err := d.GetAllServiceSummaries()
	if err != nil {
		t.Fatalf("GetAllServiceSummaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	s := summaries[0]
	if s.IsUp {
		t.Error("expected IsUp=false")
	}
	if s.ErrorMessage != "dial tcp: connection refused" {
		t.Errorf("ErrorMessage: got %q, want %q", s.ErrorMessage, "dial tcp: connection refused")
	}
}
