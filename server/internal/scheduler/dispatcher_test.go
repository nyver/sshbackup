package scheduler_test

import (
	"context"
	"testing"
	"time"

	"vpsbackupmanager/internal/domain"
	"vpsbackupmanager/internal/scheduler"
)

func dailyJob(t *testing.T, id, serverID string, policy domain.MissedRunPolicy) *domain.Job {
	t.Helper()
	j, err := domain.NewJob(time.Now(), domain.NewJobParams{
		Name:             "job-" + id,
		ServerID:         serverID,
		Enabled:          true,
		Sources:          []domain.Source{{RemotePath: "/data"}},
		Schedule:         domain.Schedule{Type: domain.ScheduleDaily, Hour: 3, Minute: 0, MissedRunPolicy: policy},
		LocalDestination: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("domain.NewJob() error = %v", err)
	}
	j.ID = id
	return j
}

func TestDispatcher_DetectMissedRuns_CatchUpCollapsesMultipleOccurrences(t *testing.T) {
	t.Parallel()
	server := &domain.Server{ID: "srv-1"}
	job := dailyJob(t, "job-1", server.ID, domain.MissedRunAsSoonAsPossible)

	runs := newMemSchedRunStore()
	// Missed three daily 03:00 occurrences (day-3, day-2, day-1) plus
	// today's, all while "the service was off".
	runs.SeedLastRun(job.ID, mustParse(t, time.RFC3339, "2026-09-05T03:00:00Z"))

	settings := newMemSettingsStore(domain.DefaultSettings())
	runner := &fakeRunner{}
	d := scheduler.NewDispatcher(
		&memJobStore{jobs: []*domain.Job{job}},
		&memServerStore{servers: map[string]*domain.Server{server.ID: server}},
		settings, runs, runner,
	)
	d.Clock = newFakeClock(mustParse(t, time.RFC3339, "2026-09-09T08:00:00Z"))

	if err := d.DetectMissedRuns(context.Background()); err != nil {
		t.Fatalf("DetectMissedRuns() error = %v", err)
	}

	if got := runner.CallCount(); got != 1 {
		t.Fatalf("runner called %d times, want exactly 1 catch-up run regardless of how many occurrences were missed", got)
	}
	calls := runner.Calls()
	if calls[0] != job.ID+":"+string(domain.TriggerMissedSchedule) {
		t.Errorf("call = %q, want trigger MISSED_SCHEDULE", calls[0])
	}
}

// TestDispatcher_DetectMissedRuns_NonUTCHostDoesNotFireGhostCatchUp
// reproduces a production bug: lastRun.StartedAt comes back from the
// store in UTC, but the schedule's Hour/Minute are local wall-clock
// values and NextRun must be called with after in the same zone as now.
// On a host east of UTC, a job that already ran this morning local time
// was wrongly flagged as having missed a further occurrence at
// "Hour:Minute UTC" (translating to a later local time on the same day)
// whenever the service restarted later that day, firing a spurious
// MISSED_SCHEDULE catch-up run.
func TestDispatcher_DetectMissedRuns_NonUTCHostDoesNotFireGhostCatchUp(t *testing.T) {
	t.Parallel()
	msk := time.FixedZone("MSK", 3*60*60)
	server := &domain.Server{ID: "srv-1"}
	job := dailyJob(t, "job-1", server.ID, domain.MissedRunAsSoonAsPossible)

	// Schedule is daily at 09:00 local (MSK). The job ran this morning at
	// 09:00 MSK, stored (like the real repository) as its UTC equivalent,
	// 06:00 UTC.
	job.Schedule.Hour, job.Schedule.Minute = 9, 0

	runs := newMemSchedRunStore()
	runs.SeedLastRun(job.ID, time.Date(2026, 9, 18, 6, 0, 0, 0, time.UTC))

	settings := newMemSettingsStore(domain.DefaultSettings())
	runner := &fakeRunner{}
	d := scheduler.NewDispatcher(
		&memJobStore{jobs: []*domain.Job{job}},
		&memServerStore{servers: map[string]*domain.Server{server.ID: server}},
		settings, runs, runner,
	)
	// The service restarts the same afternoon, well after the morning
	// run but also after "09:00 UTC" (= 12:00 MSK), the ghost occurrence
	// the pre-fix code computed.
	d.Clock = newFakeClock(time.Date(2026, 9, 18, 13, 37, 0, 0, msk))

	if err := d.DetectMissedRuns(context.Background()); err != nil {
		t.Fatalf("DetectMissedRuns() error = %v", err)
	}

	if got := runner.CallCount(); got != 0 {
		t.Fatalf("runner called %d times, want 0: this morning's local run must not be treated as missed", got)
	}
}

func TestDispatcher_DetectMissedRuns_SkipPolicyRecordsSkippedRun(t *testing.T) {
	t.Parallel()
	server := &domain.Server{ID: "srv-1"}
	job := dailyJob(t, "job-1", server.ID, domain.MissedRunSkip)

	runs := newMemSchedRunStore()
	runs.SeedLastRun(job.ID, mustParse(t, time.RFC3339, "2026-09-08T03:00:00Z"))

	settings := newMemSettingsStore(domain.DefaultSettings())
	runner := &fakeRunner{}
	d := scheduler.NewDispatcher(
		&memJobStore{jobs: []*domain.Job{job}},
		&memServerStore{servers: map[string]*domain.Server{server.ID: server}},
		settings, runs, runner,
	)
	d.Clock = newFakeClock(mustParse(t, time.RFC3339, "2026-09-09T08:00:00Z"))

	if err := d.DetectMissedRuns(context.Background()); err != nil {
		t.Fatalf("DetectMissedRuns() error = %v", err)
	}

	if runner.CallCount() != 0 {
		t.Fatalf("runner called %d times, want 0 under the SKIP policy", runner.CallCount())
	}
	created := runs.Created()
	if len(created) != 1 || created[0].Status != domain.RunSkipped {
		t.Fatalf("created runs = %+v, want exactly one SKIPPED run", created)
	}
}

func TestDispatcher_DetectMissedRuns_PausedDoesNothing(t *testing.T) {
	t.Parallel()
	server := &domain.Server{ID: "srv-1"}
	job := dailyJob(t, "job-1", server.ID, domain.MissedRunAsSoonAsPossible)

	runs := newMemSchedRunStore()
	runs.SeedLastRun(job.ID, mustParse(t, time.RFC3339, "2026-09-05T03:00:00Z"))

	settings := newMemSettingsStore(domain.DefaultSettings())
	settings.Set(domain.Settings{SchedulesPaused: true, GlobalConcurrencyLimit: 3})
	runner := &fakeRunner{}
	d := scheduler.NewDispatcher(
		&memJobStore{jobs: []*domain.Job{job}},
		&memServerStore{servers: map[string]*domain.Server{server.ID: server}},
		settings, runs, runner,
	)
	d.Clock = newFakeClock(mustParse(t, time.RFC3339, "2026-09-09T08:00:00Z"))

	if err := d.DetectMissedRuns(context.Background()); err != nil {
		t.Fatalf("DetectMissedRuns() error = %v", err)
	}
	if runner.CallCount() != 0 {
		t.Error("expected no catch-up run while schedules are paused")
	}
	if len(runs.Created()) != 0 {
		t.Error("expected no SKIPPED bookkeeping run while schedules are paused")
	}
}

func TestDispatcher_DetectMissedRuns_NoPriorRunIsLeftAlone(t *testing.T) {
	t.Parallel()
	server := &domain.Server{ID: "srv-1"}
	job := dailyJob(t, "job-1", server.ID, domain.MissedRunAsSoonAsPossible)

	settings := newMemSettingsStore(domain.DefaultSettings())
	runner := &fakeRunner{}
	d := scheduler.NewDispatcher(
		&memJobStore{jobs: []*domain.Job{job}},
		&memServerStore{servers: map[string]*domain.Server{server.ID: server}},
		settings, newMemSchedRunStore(), runner,
	)
	d.Clock = newFakeClock(mustParse(t, time.RFC3339, "2026-09-09T08:00:00Z"))

	if err := d.DetectMissedRuns(context.Background()); err != nil {
		t.Fatalf("DetectMissedRuns() error = %v", err)
	}
	if runner.CallCount() != 0 {
		t.Error("a job with no prior run should not trigger a catch-up run")
	}
}

func TestDispatcher_StartStop_PauseBlocksDispatch(t *testing.T) {
	t.Parallel()
	server := &domain.Server{ID: "srv-1"}
	job := dailyJob(t, "job-1", server.ID, domain.MissedRunAsSoonAsPossible)

	settings := newMemSettingsStore(domain.Settings{SchedulesPaused: true, GlobalConcurrencyLimit: 3})
	runner := &fakeRunner{}
	clock := newFakeClock(mustParse(t, time.RFC3339, "2026-09-08T04:00:00Z")) // after today's 03:00

	d := scheduler.NewDispatcher(
		&memJobStore{jobs: []*domain.Job{job}},
		&memServerStore{servers: map[string]*domain.Server{server.ID: server}},
		settings, newMemSchedRunStore(), runner,
	)
	d.Clock = clock
	d.PollInterval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	d.Start(ctx)
	time.Sleep(500 * time.Millisecond) // let several ticks pass while paused
	cancel()
	d.Stop()

	if runner.CallCount() != 0 {
		t.Errorf("runner called %d times, want 0 while schedules are paused", runner.CallCount())
	}
}

func TestDispatcher_StartStop_DispatchesDueJob(t *testing.T) {
	t.Parallel()
	server := &domain.Server{ID: "srv-1"}
	job := dailyJob(t, "job-1", server.ID, domain.MissedRunAsSoonAsPossible)

	settings := newMemSettingsStore(domain.DefaultSettings())
	runner := &fakeRunner{}
	clock := newFakeClock(mustParse(t, time.RFC3339, "2026-09-08T02:00:00Z")) // before today's 03:00

	d := scheduler.NewDispatcher(
		&memJobStore{jobs: []*domain.Job{job}},
		&memServerStore{servers: map[string]*domain.Server{server.ID: server}},
		settings, newMemSchedRunStore(), runner,
	)
	d.Clock = clock
	d.PollInterval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	d.Start(ctx)
	time.Sleep(200 * time.Millisecond)                            // let the dispatcher establish its baseline tick
	clock.Set(mustParse(t, time.RFC3339, "2026-09-08T03:30:00Z")) // cross the 03:00 occurrence

	ok := waitFor(t, 2*time.Second, func() bool { return runner.CallCount() > 0 })
	cancel()
	d.Stop()

	if !ok {
		t.Fatal("expected the dispatcher to run the due job")
	}
}

// TestDispatcher_StartStop_ClockJumpAcrossMultipleOccurrencesRunsOnce
// reproduces a production bug: the machine sleeps for a few days with the
// service process still resident (no restart, so DetectMissedRuns never
// runs), and the poll ticker's next real tick lands after several of the
// job's daily occurrences have passed. The live dispatcher must collapse
// that backlog into a single catch-up run, exactly like DetectMissedRuns
// does at startup, not replay one run per missed occurrence.
func TestDispatcher_StartStop_ClockJumpAcrossMultipleOccurrencesRunsOnce(t *testing.T) {
	t.Parallel()
	server := &domain.Server{ID: "srv-1"}
	job := dailyJob(t, "job-1", server.ID, domain.MissedRunAsSoonAsPossible)

	settings := newMemSettingsStore(domain.DefaultSettings())
	runner := &fakeRunner{}
	clock := newFakeClock(mustParse(t, time.RFC3339, "2026-09-10T02:00:00Z")) // before today's 03:00

	d := scheduler.NewDispatcher(
		&memJobStore{jobs: []*domain.Job{job}},
		&memServerStore{servers: map[string]*domain.Server{server.ID: server}},
		settings, newMemSchedRunStore(), runner,
	)
	d.Clock = clock
	d.PollInterval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	d.Start(ctx)
	time.Sleep(200 * time.Millisecond) // let the dispatcher establish its baseline tick

	// Jump the clock forward three days, as if the machine had been
	// asleep: three daily 03:00 occurrences (day+1, day+2, day+3) are now
	// all in the past relative to the new "now".
	clock.Set(mustParse(t, time.RFC3339, "2026-09-13T05:00:00Z"))

	ok := waitFor(t, 2*time.Second, func() bool { return runner.CallCount() > 0 })
	time.Sleep(200 * time.Millisecond) // give a wrongly-replayed second/third catch-up run a chance to appear
	cancel()
	d.Stop()

	if !ok {
		t.Fatal("expected the dispatcher to run the due job")
	}
	if got := runner.CallCount(); got != 1 {
		t.Fatalf("runner called %d times after a multi-day clock jump, want exactly 1 catch-up run regardless of how many occurrences were missed", got)
	}
}

func TestDispatcher_ConcurrencyLimits_PerServerLimitQueuesTheSecondJob(t *testing.T) {
	t.Parallel()
	server := &domain.Server{ID: "srv-1"}
	job1 := dailyJob(t, "job-1", server.ID, domain.MissedRunAsSoonAsPossible)
	job2 := dailyJob(t, "job-2", server.ID, domain.MissedRunAsSoonAsPossible)

	settings := newMemSettingsStore(domain.Settings{GlobalConcurrencyLimit: 3})
	block := make(chan struct{})
	runner := &fakeRunner{Block: block}
	clock := newFakeClock(mustParse(t, time.RFC3339, "2026-09-08T02:00:00Z"))

	d := scheduler.NewDispatcher(
		&memJobStore{jobs: []*domain.Job{job1, job2}},
		&memServerStore{servers: map[string]*domain.Server{server.ID: server}},
		settings, newMemSchedRunStore(), runner,
	)
	d.Clock = clock
	d.PollInterval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	d.Start(ctx)
	time.Sleep(200 * time.Millisecond) // let the dispatcher establish its baseline tick
	clock.Set(mustParse(t, time.RFC3339, "2026-09-08T03:30:00Z"))

	// Only one of the two same-server jobs can start (per-server limit 1);
	// this must not deadlock waiting for a second slot that will never
	// come from this same tick.
	waitFor(t, 2*time.Second, func() bool { return runner.CallCount() >= 1 })
	time.Sleep(50 * time.Millisecond) // give a wrongly-started second call a chance to appear
	if got := runner.CallCount(); got != 1 {
		t.Fatalf("runner called %d times while the first run was still active, want exactly 1 (per-server limit)", got)
	}

	close(block) // release the first run
	ok := waitFor(t, 2*time.Second, func() bool { return runner.CallCount() >= 2 })
	cancel()
	d.Stop()

	if !ok {
		t.Fatal("expected the second job to be dispatched once the server freed up, not dropped")
	}
}

// waitFor polls cond until it returns true or timeout elapses, returning
// whether cond became true. It is used only to observe background
// dispatcher activity in tests, never as a substitute for real
// synchronization in production code.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}
