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
	_, err := d.conn.Exec(
		`INSERT INTO checks (service_id, status_code, response_ms, is_up, error_message)
		 VALUES (?, ?, ?, ?, ?)`,
		serviceID, statusCode, responseMs, isUp, errVal,
	)
	return err
}

// GetAllServiceSummaries returns aggregated stats for every service.
func (d *DB) GetAllServiceSummaries() ([]ServiceSummary, error) {
	rows, err := d.conn.Query(`SELECT id, name, url FROM services ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []ServiceSummary
	for rows.Next() {
		var s ServiceSummary
		if err := rows.Scan(&s.ID, &s.Name, &s.URL); err != nil {
			return nil, err
		}
		if err := d.fillStats(&s); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

func (d *DB) fillStats(s *ServiceSummary) error {
	// Latest check.
	var checkedAt sql.NullString
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
		t, _ := time.Parse("2006-01-02 15:04:05", checkedAt.String)
		s.LastChecked = t
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
	var total, up int64
	var avgMs sql.NullFloat64
	err = d.conn.QueryRow(`
		SELECT COUNT(*), SUM(CASE WHEN is_up THEN 1 ELSE 0 END), AVG(response_ms)
		FROM checks
		WHERE service_id=? AND checked_at >= datetime('now', '-24 hours')`,
		s.ID,
	).Scan(&total, &up, &avgMs)
	if err != nil {
		return err
	}
	if total > 0 {
		s.UptimePct = float64(up) / float64(total) * 100
	}
	if avgMs.Valid {
		s.AvgResponseMs = avgMs.Float64
	}
	return nil
}

// PurgeOldChecks removes checks older than retentionDays.
func (d *DB) PurgeOldChecks(retentionDays int) error {
	_, err := d.conn.Exec(
		`DELETE FROM checks WHERE checked_at < datetime('now', ? || ' days')`,
		-retentionDays,
	)
	return err
}
