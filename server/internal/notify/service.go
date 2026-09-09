package notify

import (
	"context"
	"log/slog"

	"vpsbackupmanager/internal/domain"
)

// Service decides whether a finished run should notify the user, applies
// preferences and redaction, and delivers through Notifier. A delivery
// failure is only logged: it never changes the run it describes.
type Service struct {
	Notifier Notifier
	Redact   func(string) string // nil-safe; identity when nil
	Logger   *slog.Logger
}

func (s *Service) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func (s *Service) redact(text string) string {
	if s.Redact == nil {
		return text
	}
	return s.Redact(text)
}

// NotifyRunFinished shows a notification for run if its terminal status
// warrants one and the matching preference is enabled. It never returns
// an error: a delivery failure is logged at warning level and otherwise
// ignored, per the notifications specification.
func (s *Service) NotifyRunFinished(ctx context.Context, run *domain.Run, jobName string, prefs Preferences) {
	n, ok := buildNotification(run, jobName)
	if !ok {
		return
	}
	if n.Kind == KindSuccess && !prefs.NotifySuccess {
		return
	}
	if n.Kind == KindFailure && !prefs.NotifyFailure {
		return
	}

	n.Title = s.redact(n.Title)
	n.Body = s.redact(n.Body)

	if err := s.Notifier.Notify(ctx, n); err != nil {
		s.logger().Warn("deliver notification", "run_id", run.ID, "job_name", jobName, "error", err)
	}
}
