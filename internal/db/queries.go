package db

import (
	"database/sql"
	"time"
)

// ServiceSummary holds aggregated stats for a single service.
type ServiceSummary struct {
	ID            int64
	Name          string
	URL           string
	IsUp          bool
	UptimePct     float64
	AvgResponseMs float64
	LastChecked   time.Time
	StatusCode    int
	ErrorMessage  string
	TotalChecks   int
	History       []DayBucket // 30 entries, index 0 = oldest
}

// DayBucket holds per-day uptime data for the history bar.
type DayBucket struct {
	Date        time.Time
	TotalChecks int
	UpChecks    int
	HasData     bool
}

// UpsertService inserts or updates a service by name, returning its id.
func (d *DB) UpsertService(name, url string) (int64, error) {
	res, err := d.conn.Exec(
		`INSERT INTO services (name, url) VALUES (?, ?)
		 ON CONFLICT(name) DO UPDATE SET url=excluded.url`,
		name, url,
	)
	if err != nil {
		return 0, err
	}
	// Try last insert id; on conflict-update it may be 0, so query instead.
	id, err := res.LastInsertId()
	if err != nil || id == 0 {
		err = d.conn.QueryRow(`SELECT id FROM services WHERE name=?`, name).Scan(&id)
	}
	return id, err
}

// DeleteServicesNotIn removes services whose names are not in the provided list.
func (d *DB) DeleteServicesNotIn(names []string) error {
	if len(names) == 0 {
		_, err := d.conn.Exec(`DELETE FROM services`)
		return err
	}
	// Build placeholders dynamically.
	args := make([]interface{}, len(names))
	placeholders := "?"
	for i, n := range names {
		args[i] = n
		if i > 0 {
			placeholders += ",?"
		}
	}
	_, err := d.conn.Exec(`DELETE FROM services WHERE name NOT IN (`+placeholders+`)`, args...)
	return err
}

// InsertCheck records a single poll result.
func (d *DB) InsertCheck(serviceID int64, statusCode, responseMs int, isUp bool, errMsg string) error {
	var errVal interface{}
	if errMsg != "" {
		errVal = errMsg
	}
	// Pass checked_at explicitly from Go so the format is driver-controlled,
	// rather than relying on SQLite's datetime('now') string format.
	_, err := d.conn.Exec(
		`INSERT INTO checks (service_id, checked_at, status_code, response_ms, is_up, error_message)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		serviceID, time.Now().UTC(), statusCode, responseMs, isUp, errVal,
	)
	return err
}

// GetAllServiceSummaries returns aggregated stats for every service.
func (d *DB) GetAllServiceSummaries() ([]ServiceSummary, error) {
	rows, err := d.conn.Query(`SELECT id, name, url FROM services ORDER BY name`)
	if err != nil {
		return nil, err
	}

	// Collect all rows before closing — fillStats makes additional queries
	// and with SetMaxOpenConns(1) we can't hold a cursor open while doing that.
	var summaries []ServiceSummary
	for rows.Next() {
		var s ServiceSummary
		if err := rows.Scan(&s.ID, &s.Name, &s.URL); err != nil {
			rows.Close()
			return nil, err
		}
		summaries = append(summaries, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range summaries {
		if err := d.fillStats(&summaries[i]); err != nil {
			return nil, err
		}
		hist, err := d.GetCheckHistory(summaries[i].ID, 30)
		if err != nil {
			return nil, err
		}
		summaries[i].History = hist
	}
	return summaries, nil
}

func (d *DB) fillStats(s *ServiceSummary) error {
	// Latest check.
	var checkedAt sql.NullTime
	var statusCode sql.NullInt64
	var errMsg sql.NullString
	var isUp sql.NullBool
	err := d.conn.QueryRow(`
		SELECT checked_at, status_code, is_up, error_message
		FROM checks WHERE service_id=? ORDER BY checked_at DESC LIMIT 1`,
		s.ID,
	).Scan(&checkedAt, &statusCode, &isUp, &errMsg)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if checkedAt.Valid {
		s.LastChecked = checkedAt.Time
	}
	if statusCode.Valid {
		s.StatusCode = int(statusCode.Int64)
	}
	if isUp.Valid {
		s.IsUp = isUp.Bool
	}
	if errMsg.Valid {
		s.ErrorMessage = errMsg.String
	}

	// Uptime % and avg response over last 24 hours.
	// Use a Go time parameter so the comparison works regardless of SQLite datetime format.
	since := time.Now().UTC().Add(-24 * time.Hour)
	var total int64
	var up sql.NullInt64 // SUM returns NULL when there are no rows
	var avgMs sql.NullFloat64
	err = d.conn.QueryRow(`
		SELECT COUNT(*), SUM(CASE WHEN is_up THEN 1 ELSE 0 END), AVG(response_ms)
		FROM checks
		WHERE service_id=? AND checked_at >= ?`,
		s.ID, since,
	).Scan(&total, &up, &avgMs)
	if err != nil {
		return err
	}
	if total > 0 {
		s.UptimePct = float64(up.Int64) / float64(total) * 100
	}
	if avgMs.Valid {
		s.AvgResponseMs = avgMs.Float64
	}

	err = d.conn.QueryRow(
		`SELECT COUNT(*) FROM checks WHERE service_id = ?`, s.ID,
	).Scan(&s.TotalChecks)
	if err != nil {
		return err
	}
	return nil
}

// GetCheckHistory returns per-day uptime buckets for the last `days` days.
func (d *DB) GetCheckHistory(serviceID int64, days int) ([]DayBucket, error) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	cutoff := today.AddDate(0, 0, -(days - 1))

	rows, err := d.conn.Query(`
		SELECT substr(checked_at, 1, 10) AS day,
		       COUNT(*) AS total,
		       SUM(CASE WHEN is_up THEN 1 ELSE 0 END) AS up_count
		FROM   checks
		WHERE  service_id = ? AND checked_at >= ?
		GROUP  BY day ORDER BY day ASC`,
		serviceID, cutoff)
	if err != nil {
		return nil, err
	}
	type dbRow struct{ total, up int }
	dbData := make(map[string]dbRow)
	for rows.Next() {
		var dayStr string
		var r dbRow
		if err := rows.Scan(&dayStr, &r.total, &r.up); err != nil {
			rows.Close()
			return nil, err
		}
		dbData[dayStr] = r
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	buckets := make([]DayBucket, days)
	for i := 0; i < days; i++ {
		day := cutoff.AddDate(0, 0, i)
		b := DayBucket{Date: day}
		if r, ok := dbData[day.Format("2006-01-02")]; ok {
			b.HasData = true
			b.TotalChecks = r.total
			b.UpChecks = r.up
		}
		buckets[i] = b
	}
	return buckets, nil
}

// PurgeOldChecks removes checks older than retentionDays.
func (d *DB) PurgeOldChecks(retentionDays int) error {
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	_, err := d.conn.Exec(`DELETE FROM checks WHERE checked_at < ?`, cutoff)
	return err
}
