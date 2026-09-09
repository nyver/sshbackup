package scheduler

import (
	"context"
	"time"

	"vpsbackupmanager/internal/domain"
)

// DetectMissedRuns finds, for every enabled job with an automatic
// schedule, the earliest occurrence that should have run between its last
// recorded run and now but didn't (because the service was not running),
// and applies the job's missed-run policy: RUN_AS_SOON_AS_POSSIBLE starts
// exactly one catch-up run regardless of how many occurrences were
// missed; SKIP records a SKIPPED run instead. A job with no prior run has
// nothing to compare against and is left alone — it just hasn't run yet.
//
// Call this once at startup, before Start, and never while schedules are
// paused (paused occurrences are never queued as catch-up, per the
// scheduling specification).
func (d *Dispatcher) DetectMissedRuns(ctx context.Context) error {
	settings, err := d.Settings.Get(ctx)
	if err != nil {
		return err
	}
	if settings.SchedulesPaused {
		return nil
	}

	jobs, err := d.Jobs.List(ctx)
	if err != nil {
		return err
	}
	now := d.clock().Now()

	for _, job := range jobs {
		if !job.Enabled || !job.Schedule.IsAutomatic() {
			continue
		}
		d.detectMissedRunForJob(ctx, job, now)
	}
	return nil
}

func (d *Dispatcher) detectMissedRunForJob(ctx context.Context, job *domain.Job, now time.Time) {
	lastRun, err := d.Runs.LastRun(ctx, job.ID)
	if err != nil {
		d.logger().Error("load last run for missed-run detection", "job_id", job.ID, "error", err)
		return
	}
	if lastRun == nil {
		return
	}

	missed, found, err := NextRun(job.Schedule, lastRun.StartedAt)
	if err != nil || !found || missed.After(now) {
		return
	}

	// Establish the live dispatcher's baseline here too, so it does not
	// also fire for this same (now-handled) occurrence on its first tick.
	d.mu.Lock()
	d.scheduledUpTo[job.ID] = now
	d.mu.Unlock()

	if job.Schedule.MissedRunPolicy == domain.MissedRunSkip {
		d.recordSkippedMissedRun(ctx, job, missed)
		return
	}

	server, err := d.Servers.Get(ctx, job.ServerID)
	if err != nil {
		d.logger().Error("load server for missed-run catch-up", "job_id", job.ID, "server_id", job.ServerID, "error", err)
		return
	}
	if _, err := d.Runner.Run(ctx, job, server, domain.TriggerMissedSchedule); err != nil {
		d.logger().Error("run missed-schedule catch-up", "job_id", job.ID, "error", err)
	}
}

func (d *Dispatcher) recordSkippedMissedRun(ctx context.Context, job *domain.Job, missedAt time.Time) {
	sourcePaths := make([]string, len(job.Sources))
	for i, s := range job.Sources {
		sourcePaths[i] = s.RemotePath
	}
	run, err := domain.NewRun(missedAt, job.ID, job.ServerID, sourcePaths, domain.TriggerMissedSchedule)
	if err != nil {
		d.logger().Error("build skipped missed-run record", "job_id", job.ID, "error", err)
		return
	}
	run.Status = domain.RunSkipped
	run.ErrorMessage = "Scheduled time was missed while the service was not running"
	finished := d.clock().Now()
	run.FinishedAt = &finished
	if err := d.Runs.Create(ctx, run); err != nil {
		d.logger().Error("persist skipped missed-run record", "job_id", job.ID, "error", err)
	}
}
