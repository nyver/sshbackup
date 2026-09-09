// Package notify shows Windows toast notifications for backup run
// completion, so a user can see what happened without opening the UI.
package notify

import (
	"context"
	"fmt"
	"time"

	"vpsbackupmanager/internal/domain"
)

// Kind distinguishes the two notification preferences a user can toggle
// independently.
type Kind string

const (
	// KindSuccess covers SUCCESS and WARNING runs, governed by the
	// "success notifications" preference.
	KindSuccess Kind = "SUCCESS"
	// KindFailure covers FAILED, CANCELLED, and INTERRUPTED runs,
	// governed by the "failure notifications" preference.
	KindFailure Kind = "FAILURE"
)

// Notification is the content to show, independent of delivery mechanism.
type Notification struct {
	Kind  Kind
	Title string
	Body  string
}

// Notifier delivers one notification to the logged-in user. Delivery
// failures are the caller's concern to log; a Notifier only reports them.
type Notifier interface {
	Notify(ctx context.Context, n Notification) error
}

// Preferences are the user's independent success/failure toggles
// (domain.Settings.NotifySuccess / NotifyFailure).
type Preferences struct {
	NotifySuccess bool
	NotifyFailure bool
}

// buildNotification derives the notification content for a finished run,
// or ok=false when no notification should be shown at all (SKIPPED runs,
// or a non-terminal status reached here by mistake).
func buildNotification(run *domain.Run, jobName string) (n Notification, ok bool) {
	if !run.Status.IsTerminal() || run.Status == domain.RunSkipped {
		return Notification{}, false
	}

	switch run.Status {
	case domain.RunSuccess:
		return Notification{
			Kind:  KindSuccess,
			Title: fmt.Sprintf("Backup succeeded: %s", jobName),
			Body:  fmt.Sprintf("%s archived in %s", humanSize(run.ArchiveSize), humanDuration(run.Duration())),
		}, true

	case domain.RunWarning:
		return Notification{
			Kind:  KindSuccess,
			Title: fmt.Sprintf("Backup completed with a warning: %s", jobName),
			Body:  fmt.Sprintf("%s archived in %s. %s", humanSize(run.ArchiveSize), humanDuration(run.Duration()), run.ErrorMessage),
		}, true

	case domain.RunFailed:
		return Notification{
			Kind:  KindFailure,
			Title: fmt.Sprintf("Backup failed: %s", jobName),
			Body:  fmt.Sprintf("%s: %s%s", run.ErrorCode, run.ErrorMessage, recoverySentence(run.RecoveryOutcome)),
		}, true

	case domain.RunCancelled:
		return Notification{
			Kind:  KindFailure,
			Title: fmt.Sprintf("Backup cancelled: %s", jobName),
			Body:  "The run was cancelled." + recoverySentence(run.RecoveryOutcome),
		}, true

	case domain.RunInterrupted:
		return Notification{
			Kind:  KindFailure,
			Title: fmt.Sprintf("Backup interrupted: %s", jobName),
			Body:  "The service stopped unexpectedly while this run was in progress.",
		}, true

	default:
		return Notification{}, false
	}
}

func recoverySentence(outcome domain.RecoveryOutcome) string {
	switch outcome {
	case domain.RecoverySuccess:
		return " The recovery action succeeded."
	case domain.RecoveryFailed:
		return " The recovery action did not succeed; the remote service may still be down."
	default:
		return ""
	}
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func humanDuration(d time.Duration) string {
	return d.Round(time.Second).String()
}
