package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"vpsbackupmanager/internal/domain"
)

func secondsToDuration(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}

// idsIn builds a "col IN (?, ?, ...)" placeholder clause and the matching
// argument slice for the given job ids.
func idsIn(ids map[string]*domain.Job) (string, []any) {
	args := make([]any, 0, len(ids))
	placeholders := make([]string, 0, len(ids))
	for id := range ids {
		args = append(args, id)
		placeholders = append(placeholders, "?")
	}
	return strings.Join(placeholders, ", "), args
}

func attachSources(ctx context.Context, db *DB, jobs map[string]*domain.Job) error {
	placeholders, args := idsIn(jobs)
	rows, err := db.QueryContext(ctx, //nolint:gosec // placeholders are "?" repeated, no user input in the query text
		"SELECT id, job_id, remote_path, position, include_patterns, exclude_patterns FROM backup_sources WHERE job_id IN ("+placeholders+") ORDER BY job_id, position",
		args...)
	if err != nil {
		return fmt.Errorf("query backup sources: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			s                domain.Source
			include, exclude string
		)
		if err := rows.Scan(&s.ID, &s.JobID, &s.RemotePath, &s.Position, &include, &exclude); err != nil {
			return fmt.Errorf("scan backup source: %w", err)
		}
		s.Include = splitStrings(include)
		s.Exclude = splitStrings(exclude)
		if j, ok := jobs[s.JobID]; ok {
			j.Sources = append(j.Sources, s)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate backup sources: %w", err)
	}
	return nil
}

func attachScripts(ctx context.Context, db *DB, jobs map[string]*domain.Job) error {
	placeholders, args := idsIn(jobs)
	rows, err := db.QueryContext(ctx, //nolint:gosec // placeholders are "?" repeated, no user input in the query text
		`SELECT id, job_id, script_type, command, position, timeout_seconds, run_condition, critical_cleanup, retry_on_failure
		 FROM job_scripts WHERE job_id IN (`+placeholders+`) ORDER BY job_id, position`,
		args...)
	if err != nil {
		return fmt.Errorf("query job scripts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			sc                              domain.Script
			scriptType, runCondition        string
			criticalCleanup, retryOnFailure int64
		)
		if err := rows.Scan(&sc.ID, &sc.JobID, &scriptType, &sc.Command, &sc.Position, &sc.TimeoutSeconds,
			&runCondition, &criticalCleanup, &retryOnFailure); err != nil {
			return fmt.Errorf("scan job script: %w", err)
		}
		sc.Type = domain.ScriptType(scriptType)
		sc.RunCondition = domain.RunCondition(runCondition)
		sc.CriticalCleanup = intToBool(criticalCleanup)
		sc.RetryOnFailure = intToBool(retryOnFailure)
		if j, ok := jobs[sc.JobID]; ok {
			j.Scripts = append(j.Scripts, sc)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate job scripts: %w", err)
	}
	return nil
}

func attachSchedules(ctx context.Context, db *DB, jobs map[string]*domain.Job) error {
	placeholders, args := idsIn(jobs)
	rows, err := db.QueryContext(ctx, //nolint:gosec // placeholders are "?" repeated, no user input in the query text
		`SELECT id, job_id, schedule_type, hour, minute, weekdays, day_of_month, cron_expression, missed_run_policy
		 FROM schedules WHERE job_id IN (`+placeholders+`)`,
		args...)
	if err != nil {
		return fmt.Errorf("query schedules: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			s                          domain.Schedule
			scheduleType, missedPolicy string
			weekdays                   string
		)
		if err := rows.Scan(&s.ID, &s.JobID, &scheduleType, &s.Hour, &s.Minute, &weekdays,
			&s.DayOfMonth, &s.CronExpression, &missedPolicy); err != nil {
			return fmt.Errorf("scan schedule: %w", err)
		}
		s.Type = domain.ScheduleType(scheduleType)
		s.MissedRunPolicy = domain.MissedRunPolicy(missedPolicy)
		ints, err := splitInts(weekdays)
		if err != nil {
			return fmt.Errorf("parse schedule weekdays for job %q: %w", s.JobID, err)
		}
		for _, w := range ints {
			s.Weekdays = append(s.Weekdays, time.Weekday(w))
		}
		if j, ok := jobs[s.JobID]; ok {
			j.Schedule = s
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate schedules: %w", err)
	}
	return nil
}

func attachRetention(ctx context.Context, db *DB, jobs map[string]*domain.Job) error {
	placeholders, args := idsIn(jobs)
	rows, err := db.QueryContext(ctx, //nolint:gosec // placeholders are "?" repeated, no user input in the query text
		"SELECT id, job_id, keep_last, max_age_days FROM retention_policies WHERE job_id IN ("+placeholders+")",
		args...)
	if err != nil {
		return fmt.Errorf("query retention policies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var p domain.RetentionPolicy
		var keepLast, maxAgeDays sql.NullInt64
		if err := rows.Scan(&p.ID, &p.JobID, &keepLast, &maxAgeDays); err != nil {
			return fmt.Errorf("scan retention policy: %w", err)
		}
		p.KeepLast = intPtr(keepLast)
		p.MaxAgeDays = intPtr(maxAgeDays)
		if j, ok := jobs[p.JobID]; ok {
			j.RetentionPolicy = p
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate retention policies: %w", err)
	}
	return nil
}
