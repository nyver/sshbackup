package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"vpsbackupmanager/internal/domain"
)

// DefaultPollInterval is how often the dispatcher checks for due jobs.
// Schedules have minute resolution, so this stays well under a minute.
const DefaultPollInterval = 15 * time.Second

// Dispatcher drives due jobs from persisted schedules under the global,
// per-server, and per-job concurrency limits, with an explicit owner
// goroutine and shutdown path.
type Dispatcher struct {
	Jobs         JobStore
	Servers      ServerStore
	Settings     SettingsStore
	Runs         RunStore
	Runner       Runner
	Clock        Clock
	PollInterval time.Duration
	Logger       *slog.Logger

	mu             sync.Mutex
	scheduledUpTo  map[string]time.Time // jobID -> latest occurrence already considered
	runningCount   int
	runningServers map[string]int

	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// NewDispatcher constructs a Dispatcher. Call Start to begin dispatching
// and Stop to shut it down.
func NewDispatcher(jobs JobStore, servers ServerStore, settings SettingsStore, runs RunStore, runner Runner) *Dispatcher {
	return &Dispatcher{
		Jobs: jobs, Servers: servers, Settings: settings, Runs: runs, Runner: runner,
		scheduledUpTo:  make(map[string]time.Time),
		runningServers: make(map[string]int),
		stopCh:         make(chan struct{}),
		doneCh:         make(chan struct{}),
	}
}

func (d *Dispatcher) clock() Clock {
	if d.Clock != nil {
		return d.Clock
	}
	return SystemClock{}
}

func (d *Dispatcher) logger() *slog.Logger {
	if d.Logger != nil {
		return d.Logger
	}
	return slog.Default()
}

func (d *Dispatcher) pollInterval() time.Duration {
	if d.PollInterval > 0 {
		return d.PollInterval
	}
	return DefaultPollInterval
}

// Start begins the dispatch loop in its own goroutine, owned by this
// Dispatcher: only Stop (or ctx cancellation) ends it.
func (d *Dispatcher) Start(ctx context.Context) {
	go d.loop(ctx)
}

// Stop ends the dispatch loop and waits for it to exit. It does not wait
// for in-flight job runs to finish; that is the caller's (graceful
// shutdown's) responsibility.
func (d *Dispatcher) Stop() {
	d.stopOnce.Do(func() { close(d.stopCh) })
	<-d.doneCh
}

func (d *Dispatcher) loop(ctx context.Context) {
	defer close(d.doneCh)
	ticker := time.NewTicker(d.pollInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.tick(ctx)
		}
	}
}

func (d *Dispatcher) tick(ctx context.Context) {
	settings, err := d.Settings.Get(ctx)
	if err != nil {
		d.logger().Error("read settings for dispatch tick", "error", err)
		return
	}
	if settings.SchedulesPaused {
		return
	}

	jobs, err := d.Jobs.List(ctx)
	if err != nil {
		d.logger().Error("list jobs for dispatch tick", "error", err)
		return
	}

	now := d.clock().Now()
	for _, job := range jobs {
		if !job.Enabled || !job.Schedule.IsAutomatic() {
			continue
		}
		d.considerJob(ctx, job, now, settings.GlobalConcurrencyLimit)
	}
}

// considerJob checks whether job has crossed a schedule occurrence since
// the last time it was considered and, if so, tries to start it. The
// per-job baseline advances only once the job actually starts, and it
// advances all the way to now rather than to the occurrence that fired:
// if the poll loop fell behind by more than one occurrence (e.g. the host
// slept for a few days while this process stayed resident, so no tick ran
// and DetectMissedRuns never got a chance to collapse the backlog at
// startup), the whole backlog counts as caught up after one run, matching
// "at most one catch-up run per job regardless of how many occurrences
// were missed" instead of replaying one run per missed occurrence on
// successive ticks. If a concurrency limit blocks the start, the baseline
// does not advance and the same due occurrence is retried on the next
// tick instead of being silently dropped, per "a job that cannot start
// because of a limit SHALL wait in the dispatch queue rather than fail."
// The first tick for a newly seen job only establishes the baseline:
// catching up on occurrences missed before the service started is
// DetectMissedRuns's job, not the live dispatcher's.
func (d *Dispatcher) considerJob(ctx context.Context, job *domain.Job, now time.Time, globalLimit int) {
	d.mu.Lock()
	last, seen := d.scheduledUpTo[job.ID]
	if !seen {
		d.scheduledUpTo[job.ID] = now
		d.mu.Unlock()
		return
	}
	d.mu.Unlock()

	next, ok, err := NextRun(job.Schedule, last)
	if err != nil || !ok || next.After(now) {
		return
	}

	if d.tryStart(ctx, job, domain.TriggerSchedule, globalLimit) {
		d.mu.Lock()
		d.scheduledUpTo[job.ID] = now
		d.mu.Unlock()
	}
}

// tryStart starts job if the global and per-server limits allow it,
// reporting whether it did. A job blocked by its own database lock is
// handled inside Runner (recorded SKIPPED), not here.
func (d *Dispatcher) tryStart(ctx context.Context, job *domain.Job, trigger domain.Trigger, globalLimit int) bool {
	server, err := d.Servers.Get(ctx, job.ServerID)
	if err != nil {
		d.logger().Error("load server for due job", "job_id", job.ID, "server_id", job.ServerID, "error", err)
		return false
	}

	d.mu.Lock()
	if d.runningCount >= globalLimit {
		d.mu.Unlock()
		d.logger().Info("job due but global concurrency limit reached; retrying next tick", "job_id", job.ID)
		return false
	}
	if d.runningServers[job.ServerID] > 0 {
		d.mu.Unlock()
		d.logger().Info("job due but its server already has an active run; retrying next tick", "job_id", job.ID)
		return false
	}
	d.runningCount++
	d.runningServers[job.ServerID]++
	d.mu.Unlock()

	go d.runAndRelease(ctx, job, server, trigger)
	return true
}

func (d *Dispatcher) runAndRelease(ctx context.Context, job *domain.Job, server *domain.Server, trigger domain.Trigger) {
	defer func() {
		d.mu.Lock()
		d.runningCount--
		d.runningServers[job.ServerID]--
		d.mu.Unlock()
	}()
	if _, err := d.Runner.Run(ctx, job, server, trigger); err != nil {
		d.logger().Error("run dispatched job", "job_id", job.ID, "error", err)
	}
}
