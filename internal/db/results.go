package db

import (
	"fmt"
	"time"
)

// Result represents a single dependency check result.
type Result struct {
	ID              int64
	ScanID          int64
	Source          string
	Chart           string
	Dependency      string
	Type            string
	Protocol        string
	CurrentVersion  string
	LatestVersion   string
	Scope           string
	UpdateAvailable bool
	CheckedAt       time.Time
}

// InsertResult inserts a single result for a scan.
func (db *DB) InsertResult(r *Result) error {
	result, err := db.conn.Exec(
		`INSERT INTO results (scan_id, source, chart, dependency, type, protocol,
		                      current_version, latest_version, scope, update_available, checked_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ScanID, r.Source, r.Chart, r.Dependency, r.Type, r.Protocol,
		r.CurrentVersion, r.LatestVersion, r.Scope, r.UpdateAvailable, r.CheckedAt,
	)
	if err != nil {
		return fmt.Errorf("insert result: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	r.ID = id
	return nil
}

// InsertResults inserts multiple results in a single transaction.
func (db *DB) InsertResults(results []Result) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO results (scan_id, source, chart, dependency, type, protocol,
		                      current_version, latest_version, scope, update_available, checked_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, r := range results {
		_, err := stmt.Exec(
			r.ScanID, r.Source, r.Chart, r.Dependency, r.Type, r.Protocol,
			r.CurrentVersion, r.LatestVersion, r.Scope, r.UpdateAvailable, r.CheckedAt,
		)
		if err != nil {
			return fmt.Errorf("insert result: %w", err)
		}
	}

	return tx.Commit()
}

// GetResultsByScan retrieves all results for a specific scan.
func (db *DB) GetResultsByScan(scanID int64) ([]Result, error) {
	rows, err := db.conn.Query(
		`SELECT id, scan_id, source, chart, dependency, type, protocol,
		        current_version, latest_version, scope, update_available, checked_at
		 FROM results WHERE scan_id = ? ORDER BY source, chart, dependency`,
		scanID,
	)
	if err != nil {
		return nil, fmt.Errorf("query results: %w", err)
	}
	defer rows.Close()

	return scanResultRows(rows)
}

// GetLatestResults retrieves results from the most recent completed scan.
func (db *DB) GetLatestResults() ([]Result, error) {
	rows, err := db.conn.Query(
		`SELECT r.id, r.scan_id, r.source, r.chart, r.dependency, r.type, r.protocol,
		        r.current_version, r.latest_version, r.scope, r.update_available, r.checked_at
		 FROM results r
		 JOIN scans s ON r.scan_id = s.id
		 WHERE s.status = ?
		 ORDER BY s.started_at DESC, r.source, r.chart, r.dependency
		 LIMIT (SELECT result_count FROM scans WHERE status = ? ORDER BY started_at DESC LIMIT 1)`,
		ScanStatusCompleted, ScanStatusCompleted,
	)
	if err != nil {
		return nil, fmt.Errorf("query latest results: %w", err)
	}
	defer rows.Close()

	return scanResultRows(rows)
}

// CountUpdatesAvailable returns the number of dependencies with updates in a scan.
func (db *DB) CountUpdatesAvailable(scanID int64) (int, error) {
	var count int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM results WHERE scan_id = ? AND update_available = 1`,
		scanID,
	).Scan(&count)
	return count, err
}

// GetResultStats returns basic statistics for a scan.
type ResultStats struct {
	TotalResults     int
	UpdatesAvailable int
	UpToDate         int
	UniqueCharts     int
	UniqueSources    int
}

// GetResultStats returns statistics for a scan.
func (db *DB) GetResultStats(scanID int64) (*ResultStats, error) {
	var stats ResultStats

	err := db.conn.QueryRow(
		`SELECT
			COUNT(*) as total,
			SUM(CASE WHEN update_available = 1 THEN 1 ELSE 0 END) as updates,
			COUNT(DISTINCT chart) as charts,
			COUNT(DISTINCT source) as sources
		 FROM results WHERE scan_id = ?`,
		scanID,
	).Scan(&stats.TotalResults, &stats.UpdatesAvailable, &stats.UniqueCharts, &stats.UniqueSources)
	if err != nil {
		return nil, fmt.Errorf("get result stats: %w", err)
	}

	stats.UpToDate = stats.TotalResults - stats.UpdatesAvailable
	return &stats, nil
}

func scanResultRows(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]Result, error) {
	var results []Result
	for rows.Next() {
		var r Result
		var latest *string
		if err := rows.Scan(
			&r.ID, &r.ScanID, &r.Source, &r.Chart, &r.Dependency, &r.Type, &r.Protocol,
			&r.CurrentVersion, &latest, &r.Scope, &r.UpdateAvailable, &r.CheckedAt,
		); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		if latest != nil {
			r.LatestVersion = *latest
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
