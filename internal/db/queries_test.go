package db

import (
	"testing"
	"time"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// mustInsertCheck inserts a check at time.Now().UTC().Add(offset).
func mustInsertCheck(t *testing.T, d *DB, svcID int64, isUp bool, responseMs int, offset time.Duration) {
	t.Helper()
	checkedAt := time.Now().UTC().Add(offset)
	_, err := d.conn.Exec(
		`INSERT INTO checks (service_id, checked_at, status_code, response_ms, is_up) VALUES (?, ?, ?, ?, ?)`,
		svcID, checkedAt, 200, responseMs, isUp,
	)
	if err != nil {
		t.Fatalf("mustInsertCheck: %v", err)
	}
}

func TestUpsertService_Insert(t *testing.T) {
	d := newTestDB(t)
	id, err := d.UpsertService("Alpha", "https://alpha.example.com")
	if err != nil {
		t.Fatalf("UpsertService: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero ID")
	}
}

func TestUpsertService_Update(t *testing.T) {
	d := newTestDB(t)
	id1, err := d.UpsertService("Alpha", "https://old.example.com")
	if err != nil {
		t.Fatalf("first UpsertService: %v", err)
	}
	id2, err := d.UpsertService("Alpha", "https://new.example.com")
	if err != nil {
		t.Fatalf("second UpsertService: %v", err)
	}
	if id1 != id2 {
		t.Errorf("expected same ID on update, got %d then %d", id1, id2)
	}
	var url string
	if err := d.conn.QueryRow(`SELECT url FROM services WHERE id=?`, id1).Scan(&url); err != nil {
		t.Fatalf("query url: %v", err)
	}
	if url != "https://new.example.com" {
		t.Errorf("URL: got %q, want %q", url, "https://new.example.com")
	}
}

func TestDeleteServicesNotIn(t *testing.T) {
	d := newTestDB(t)
	if _, err := d.UpsertService("Keep", "https://keep.example.com"); err != nil {
		t.Fatalf("UpsertService Keep: %v", err)
	}
	if _, err := d.UpsertService("Remove", "https://remove.example.com"); err != nil {
		t.Fatalf("UpsertService Remove: %v", err)
	}

	if err := d.DeleteServicesNotIn([]string{"Keep"}); err != nil {
		t.Fatalf("DeleteServicesNotIn: %v", err)
	}

	var count int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM services WHERE name='Remove'`).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 0 {
		t.Error("'Remove' service should have been deleted")
	}

	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM services WHERE name='Keep'`).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Error("'Keep' service should still exist")
	}
}

func TestDeleteServicesNotIn_Empty(t *testing.T) {
	d := newTestDB(t)
	if _, err := d.UpsertService("Alpha", "https://alpha.example.com"); err != nil {
		t.Fatalf("UpsertService: %v", err)
	}

	if err := d.DeleteServicesNotIn([]string{}); err != nil {
		t.Fatalf("DeleteServicesNotIn: %v", err)
	}

	var count int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM services`).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 0 {
		t.Errorf("expected all services deleted, got %d", count)
	}
}

func TestInsertCheck(t *testing.T) {
	d := newTestDB(t)
	id, err := d.UpsertService("Alpha", "https://alpha.example.com")
	if err != nil {
		t.Fatalf("UpsertService: %v", err)
	}

	if err := d.InsertCheck(id, 200, 150, true, ""); err != nil {
		t.Fatalf("InsertCheck: %v", err)
	}

	summaries, err := d.GetAllServiceSummaries()
	if err != nil {
		t.Fatalf("GetAllServiceSummaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].TotalChecks != 1 {
		t.Errorf("TotalChecks: got %d, want 1", summaries[0].TotalChecks)
	}
}

func TestGetAllServiceSummaries_NoChecks(t *testing.T) {
	d := newTestDB(t)
	if _, err := d.UpsertService("Alpha", "https://alpha.example.com"); err != nil {
		t.Fatalf("UpsertService: %v", err)
	}

	summaries, err := d.GetAllServiceSummaries()
	if err != nil {
		t.Fatalf("GetAllServiceSummaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	s := summaries[0]
	if s.UptimePct != 0 {
		t.Errorf("UptimePct: got %f, want 0", s.UptimePct)
	}
	if s.IsUp {
		t.Error("IsUp: expected false for service with no checks")
	}
}

func TestGetAllServiceSummaries_UptimePct(t *testing.T) {
	d := newTestDB(t)
	id, err := d.UpsertService("Alpha", "https://alpha.example.com")
	if err != nil {
		t.Fatalf("UpsertService: %v", err)
	}

	mustInsertCheck(t, d, id, true, 100, -1*time.Hour)
	mustInsertCheck(t, d, id, true, 100, -2*time.Hour)
	mustInsertCheck(t, d, id, true, 100, -3*time.Hour)
	mustInsertCheck(t, d, id, false, 0, -4*time.Hour)

	summaries, err := d.GetAllServiceSummaries()
	if err != nil {
		t.Fatalf("GetAllServiceSummaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if got := summaries[0].UptimePct; got != 75.0 {
		t.Errorf("UptimePct: got %f, want 75.0", got)
	}
}

func TestGetCheckHistory_NoData(t *testing.T) {
	d := newTestDB(t)
	id, err := d.UpsertService("Alpha", "https://alpha.example.com")
	if err != nil {
		t.Fatalf("UpsertService: %v", err)
	}

	hist, err := d.GetCheckHistory(id, 30)
	if err != nil {
		t.Fatalf("GetCheckHistory: %v", err)
	}
	if len(hist) != 30 {
		t.Fatalf("expected 30 buckets, got %d", len(hist))
	}
	for i, b := range hist {
		if b.HasData {
			t.Errorf("bucket[%d]: expected HasData=false", i)
		}
	}
}

func TestGetCheckHistory_Today(t *testing.T) {
	d := newTestDB(t)
	id, err := d.UpsertService("Alpha", "https://alpha.example.com")
	if err != nil {
		t.Fatalf("UpsertService: %v", err)
	}

	mustInsertCheck(t, d, id, true, 100, -1*time.Minute)
	mustInsertCheck(t, d, id, true, 100, -2*time.Minute)
	mustInsertCheck(t, d, id, false, 0, -3*time.Minute)

	hist, err := d.GetCheckHistory(id, 30)
	if err != nil {
		t.Fatalf("GetCheckHistory: %v", err)
	}
	last := hist[29]
	if !last.HasData {
		t.Error("last bucket: expected HasData=true for today's checks")
	}
	if last.TotalChecks != 3 {
		t.Errorf("last bucket TotalChecks: got %d, want 3", last.TotalChecks)
	}
	if last.UpChecks != 2 {
		t.Errorf("last bucket UpChecks: got %d, want 2", last.UpChecks)
	}
}

func TestGetCheckHistory_Gap(t *testing.T) {
	d := newTestDB(t)
	id, err := d.UpsertService("Alpha", "https://alpha.example.com")
	if err != nil {
		t.Fatalf("UpsertService: %v", err)
	}

	// Check on day 0 (29 days ago) and day 29 (today)
	mustInsertCheck(t, d, id, true, 100, -29*24*time.Hour)
	mustInsertCheck(t, d, id, true, 100, -1*time.Minute)

	hist, err := d.GetCheckHistory(id, 30)
	if err != nil {
		t.Fatalf("GetCheckHistory: %v", err)
	}
	if !hist[0].HasData {
		t.Error("bucket[0]: expected HasData=true (29 days ago)")
	}
	if !hist[29].HasData {
		t.Error("bucket[29]: expected HasData=true (today)")
	}
	for i := 1; i < 29; i++ {
		if hist[i].HasData {
			t.Errorf("bucket[%d]: expected HasData=false (gap)", i)
		}
	}
}

func TestPurgeOldChecks(t *testing.T) {
	d := newTestDB(t)
	id, err := d.UpsertService("Alpha", "https://alpha.example.com")
	if err != nil {
		t.Fatalf("UpsertService: %v", err)
	}

	mustInsertCheck(t, d, id, true, 100, -100*24*time.Hour)
	mustInsertCheck(t, d, id, true, 100, -1*time.Hour)

	if err := d.PurgeOldChecks(90); err != nil {
		t.Fatalf("PurgeOldChecks: %v", err)
	}

	var count int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM checks WHERE service_id=?`, id).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 check remaining after purge, got %d", count)
	}
}
