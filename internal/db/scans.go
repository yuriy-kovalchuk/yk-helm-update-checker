package db

import (
	"database/sql"
	"fmt"
	"time"
)

// ScanStatus represents the state of a scan.
type ScanStatus string

const (
	ScanStatusRunning   ScanStatus = "running"
	ScanStatusCompleted ScanStatus = "completed"
	ScanStatusFailed    ScanStatus = "failed"
)

// ScanTrigger indicates how the scan was initiated.
type ScanTrigger string

const (
	ScanTriggerManual    ScanTrigger = "manual"
	ScanTriggerScheduled ScanTrigger = "scheduled"
)

// Scan represents a scan execution record.
type Scan struct {
	ID               int64
	StartedAt        time.Time
	CompletedAt      *time.Time
	Status           ScanStatus
	ErrorMessage     string
	ResultCount      int
	UpdatesAvailable int
	Scope            string
	Trigger          ScanTrigger
}

// CreateScan creates a new scan record with status "running".
func (db *DB) CreateScan(scope string, trigger ScanTrigger) (int64, error) {
	result, err := db.conn.Exec(
		`INSERT INTO scans (scope, trigger, status) VALUES (?, ?, ?)`,
		scope, trigger, ScanStatusRunning,
	)
	if err != nil {
		return 0, fmt.Errorf("insert scan: %w", err)
	}
	return result.LastInsertId()
}

// CompleteScan marks a scan as completed with the result count.
func (db *DB) CompleteScan(scanID int64, resultCount int) error {
	_, err := db.conn.Exec(
		`UPDATE scans SET status = ?, completed_at = CURRENT_TIMESTAMP, result_count = ? WHERE id = ?`,
		ScanStatusCompleted, resultCount, scanID,
	)
	return err
}

// FailScan marks a scan as failed with an error message.
func (db *DB) FailScan(scanID int64, errMsg string) error {
	_, err := db.conn.Exec(
		`UPDATE scans SET status = ?, completed_at = CURRENT_TIMESTAMP, error_message = ? WHERE id = ?`,
		ScanStatusFailed, errMsg, scanID,
	)
	return err
}

// GetScan retrieves a scan by ID.
func (db *DB) GetScan(scanID int64) (*Scan, error) {
	var s Scan
	var completedAt sql.NullTime
	var errMsg sql.NullString

	err := db.conn.QueryRow(
		`SELECT id, started_at, completed_at, status, error_message, result_count, scope, trigger
		 FROM scans WHERE id = ?`,
		scanID,
	).Scan(&s.ID, &s.StartedAt, &completedAt, &s.Status, &errMsg, &s.ResultCount, &s.Scope, &s.Trigger)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get scan: %w", err)
	}

	if completedAt.Valid {
		s.CompletedAt = &completedAt.Time
	}
	s.ErrorMessage = errMsg.String

	return &s, nil
}

// ListScans returns scans ordered by started_at descending, with updates_available
// computed from the results table. Pass limit <= 0 for no limit.
func (db *DB) ListScans(limit int) ([]Scan, error) {
	query := `SELECT s.id, s.started_at, s.completed_at, s.status, s.error_message,
	                 s.result_count, s.scope, s.trigger,
	                 COALESCE(SUM(r.update_available), 0) AS updates_available
	          FROM scans s
	          LEFT JOIN results r ON r.scan_id = s.id
	          GROUP BY s.id
	          ORDER BY s.started_at DESC`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("list scans: %w", err)
	}
	defer rows.Close()

	var scans []Scan
	for rows.Next() {
		var s Scan
		var completedAt sql.NullTime
		var errMsg sql.NullString

		if err := rows.Scan(&s.ID, &s.StartedAt, &completedAt, &s.Status, &errMsg,
			&s.ResultCount, &s.Scope, &s.Trigger, &s.UpdatesAvailable); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		if completedAt.Valid {
			s.CompletedAt = &completedAt.Time
		}
		s.ErrorMessage = errMsg.String

		scans = append(scans, s)
	}

	return scans, rows.Err()
}

// LatestScan returns the most recent scan or nil if none exist.
func (db *DB) LatestScan() (*Scan, error) {
	scans, err := db.ListScans(1)
	if err != nil {
		return nil, err
	}
	if len(scans) == 0 {
		return nil, nil
	}
	return &scans[0], nil
}

// LatestCompletedScan returns the most recent completed scan or nil if none exist.
func (db *DB) LatestCompletedScan() (*Scan, error) {
	var s Scan
	var completedAt sql.NullTime
	var errMsg sql.NullString

	err := db.conn.QueryRow(
		`SELECT id, started_at, completed_at, status, error_message, result_count, scope, trigger
		 FROM scans WHERE status = ? ORDER BY started_at DESC LIMIT 1`,
		ScanStatusCompleted,
	).Scan(&s.ID, &s.StartedAt, &completedAt, &s.Status, &errMsg, &s.ResultCount, &s.Scope, &s.Trigger)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest completed scan: %w", err)
	}

	if completedAt.Valid {
		s.CompletedAt = &completedAt.Time
	}
	s.ErrorMessage = errMsg.String

	return &s, nil
}

// IsScanning returns true if there's a scan currently running.
func (db *DB) IsScanning() (bool, error) {
	var count int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM scans WHERE status = ?`,
		ScanStatusRunning,
	).Scan(&count)
	return count > 0, err
}

// FailStuckScans auto-fails scans that have been in "running" state longer than olderThan.
// Returns the number of scans updated.
func (db *DB) FailStuckScans(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := db.conn.Exec(
		`UPDATE scans SET status = ?, completed_at = CURRENT_TIMESTAMP, error_message = ?
		 WHERE status = ? AND started_at < ?`,
		ScanStatusFailed, "timed out: job was likely killed before completing", ScanStatusRunning, cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("fail stuck scans: %w", err)
	}
	return result.RowsAffected()
}

// DeleteOldScans removes scans older than the given duration, keeping at least minKeep scans.
func (db *DB) DeleteOldScans(olderThan time.Duration, minKeep int) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	// Get IDs of scans to keep (most recent minKeep)
	result, err := db.conn.Exec(
		`DELETE FROM scans WHERE started_at < ? AND id NOT IN (
			SELECT id FROM scans ORDER BY started_at DESC LIMIT ?
		)`,
		cutoff, minKeep,
	)
	if err != nil {
		return 0, fmt.Errorf("delete old scans: %w", err)
	}

	return result.RowsAffected()
}
