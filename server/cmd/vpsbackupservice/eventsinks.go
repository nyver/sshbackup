package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"vpsbackupmanager/internal/applog"
	"vpsbackupmanager/internal/domain"
	"vpsbackupmanager/internal/notify"
	"vpsbackupmanager/internal/store"
)

// compositeEventSink fans one backup.EventSink call out to several,
// so the composition root can wire IPC event delivery, per-run log
// files, and shutdown run-tracking without any of them knowing about
// the others.
type compositeEventSink struct {
	sinks []eventSink
}

// eventSink mirrors backup.EventSink; declared locally so this file does
// not need to import internal/backup solely for the interface name.
type eventSink interface {
	RunStarted(run *domain.Run)
	StepChanged(runID string, step *domain.RunStep)
	RunFinished(run *domain.Run)
}

func newCompositeEventSink(sinks ...eventSink) *compositeEventSink {
	return &compositeEventSink{sinks: sinks}
}

func (c *compositeEventSink) RunStarted(run *domain.Run) {
	for _, s := range c.sinks {
		s.RunStarted(run)
	}
}

func (c *compositeEventSink) StepChanged(runID string, step *domain.RunStep) {
	for _, s := range c.sinks {
		s.StepChanged(runID, step)
	}
}

func (c *compositeEventSink) RunFinished(run *domain.Run) {
	for _, s := range c.sinks {
		s.RunFinished(run)
	}
}

// waitGroupEventSink tracks active runs so graceful shutdown can wait for
// them (bounded by Engine.CleanupBudget's own detached-context timeout)
// instead of exiting mid-cleanup.
type waitGroupEventSink struct {
	wg *sync.WaitGroup
}

func newWaitGroupEventSink(wg *sync.WaitGroup) *waitGroupEventSink {
	return &waitGroupEventSink{wg: wg}
}

func (w *waitGroupEventSink) RunStarted(*domain.Run)              { w.wg.Add(1) }
func (w *waitGroupEventSink) StepChanged(string, *domain.RunStep) {}
func (w *waitGroupEventSink) RunFinished(*domain.Run)             { w.wg.Done() }

// runLogEventSink appends a timestamped line per step change to
// logs/runs/<run-id>.log, and closes that file when the run finishes, per
// the run-history specification's log-separation requirement.
type runLogEventSink struct {
	dataDir string
	logger  *slog.Logger

	mu    sync.Mutex
	files map[string]*os.File
}

func newRunLogEventSink(dataDir string, logger *slog.Logger) *runLogEventSink {
	return &runLogEventSink{dataDir: dataDir, logger: logger, files: make(map[string]*os.File)}
}

func (s *runLogEventSink) RunStarted(run *domain.Run) {
	f, err := applog.OpenRunLog(s.dataDir, run.ID)
	if err != nil {
		s.logger.Error("open run log", "run_id", run.ID, "error", err)
		return
	}
	s.mu.Lock()
	s.files[run.ID] = f
	s.mu.Unlock()
	s.writeLine(run.ID, fmt.Sprintf("run started: job_id=%s trigger=%s", run.JobID, run.Trigger))
}

func (s *runLogEventSink) StepChanged(runID string, step *domain.RunStep) {
	s.writeLine(runID, fmt.Sprintf("step %s: status=%s duration=%s", step.Type, step.Status, step.Duration))
	if step.Error != "" {
		s.writeLine(runID, fmt.Sprintf("  error: %s", step.Error))
	}
}

func (s *runLogEventSink) RunFinished(run *domain.Run) {
	s.writeLine(run.ID, fmt.Sprintf("run finished: status=%s recovery=%s", run.Status, run.RecoveryOutcome))

	s.mu.Lock()
	f, ok := s.files[run.ID]
	delete(s.files, run.ID)
	s.mu.Unlock()
	if ok {
		if err := f.Close(); err != nil {
			s.logger.Error("close run log", "run_id", run.ID, "error", err)
		}
	}
}

func (s *runLogEventSink) writeLine(runID, line string) {
	s.mu.Lock()
	f, ok := s.files[runID]
	s.mu.Unlock()
	if !ok {
		return
	}
	timestamped := fmt.Sprintf("%s %s\n", time.Now().UTC().Format(time.RFC3339), line)
	if _, err := f.Write([]byte(timestamped)); err != nil {
		s.logger.Error("write run log", "run_id", runID, "error", err)
	}
}

// notifyEventSink shows a toast notification when a run finishes,
// resolving the job name and the user's current notification preferences
// at that moment.
type notifyEventSink struct {
	notify   *notify.Service
	jobs     *store.JobRepository
	settings *store.SettingsRepository
	logger   *slog.Logger
}

func newNotifyEventSink(n *notify.Service, jobs *store.JobRepository, settings *store.SettingsRepository, logger *slog.Logger) *notifyEventSink {
	return &notifyEventSink{notify: n, jobs: jobs, settings: settings, logger: logger}
}

func (s *notifyEventSink) RunStarted(*domain.Run)              {}
func (s *notifyEventSink) StepChanged(string, *domain.RunStep) {}

func (s *notifyEventSink) RunFinished(run *domain.Run) {
	ctx := context.Background()
	jobName := run.JobID
	if job, err := s.jobs.Get(ctx, run.JobID); err == nil {
		jobName = job.Name
	}
	settings, err := s.settings.Get(ctx)
	if err != nil {
		s.logger.Warn("load settings for notification preferences", "run_id", run.ID, "error", err)
		return
	}
	s.notify.NotifyRunFinished(ctx, run, jobName, notify.Preferences{
		NotifySuccess: settings.NotifySuccess, NotifyFailure: settings.NotifyFailure,
	})
}
