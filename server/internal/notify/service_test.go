package notify

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"vpsbackupmanager/internal/domain"
)

type fakeNotifier struct {
	sent []Notification
	err  error
}

func (f *fakeNotifier) Notify(_ context.Context, n Notification) error {
	f.sent = append(f.sent, n)
	return f.err
}

func testRun(status domain.RunStatus) *domain.Run {
	start := time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC)
	finish := start.Add(90 * time.Second)
	return &domain.Run{
		ID: "run-1", JobID: "job-1", Status: status,
		StartedAt: start, FinishedAt: &finish,
		ArchiveSize: 4200000, ArchiveName: "beresta_2026-01-01_03-00-00.tar.gz",
	}
}

func TestService_NotifyRunFinished_ContentPerStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		run         *domain.Run
		wantKind    Kind
		wantInTitle string
		wantInBody  string
	}{
		{
			name:        "success",
			run:         testRun(domain.RunSuccess),
			wantKind:    KindSuccess,
			wantInTitle: "succeeded",
			wantInBody:  "archived in",
		},
		{
			name: "warning includes the caveat",
			run: func() *domain.Run {
				r := testRun(domain.RunWarning)
				r.ErrorMessage = "health check failed after 3 attempt(s)"
				return r
			}(),
			wantKind:    KindSuccess,
			wantInTitle: "warning",
			wantInBody:  "health check failed",
		},
		{
			name: "failure includes stage, message, and recovery outcome",
			run: func() *domain.Run {
				r := testRun(domain.RunFailed)
				r.ErrorCode = domain.ErrArchiveFailed
				r.ErrorMessage = "no space left on device"
				r.RecoveryOutcome = domain.RecoverySuccess
				return r
			}(),
			wantKind:    KindFailure,
			wantInTitle: "failed",
			wantInBody:  "recovery action succeeded",
		},
		{
			name: "failure with failed recovery warns the service may still be down",
			run: func() *domain.Run {
				r := testRun(domain.RunFailed)
				r.RecoveryOutcome = domain.RecoveryFailed
				return r
			}(),
			wantKind:    KindFailure,
			wantInTitle: "failed",
			wantInBody:  "may still be down",
		},
		{
			name:        "cancelled",
			run:         testRun(domain.RunCancelled),
			wantKind:    KindFailure,
			wantInTitle: "cancelled",
			wantInBody:  "cancelled",
		},
		{
			name:        "interrupted",
			run:         testRun(domain.RunInterrupted),
			wantKind:    KindFailure,
			wantInTitle: "interrupted",
			wantInBody:  "stopped unexpectedly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			notifier := &fakeNotifier{}
			s := &Service{Notifier: notifier, Logger: silentLogger()}
			s.NotifyRunFinished(context.Background(), tt.run, "beresta", Preferences{NotifySuccess: true, NotifyFailure: true})

			if len(notifier.sent) != 1 {
				t.Fatalf("sent %d notifications, want 1", len(notifier.sent))
			}
			got := notifier.sent[0]
			if got.Kind != tt.wantKind {
				t.Errorf("Kind = %q, want %q", got.Kind, tt.wantKind)
			}
			if !strings.Contains(got.Title, tt.wantInTitle) {
				t.Errorf("Title = %q, want it to contain %q", got.Title, tt.wantInTitle)
			}
			if !strings.Contains(got.Body, tt.wantInBody) {
				t.Errorf("Body = %q, want it to contain %q", got.Body, tt.wantInBody)
			}
		})
	}
}

func TestService_NotifyRunFinished_SkippedRunIsQuiet(t *testing.T) {
	t.Parallel()
	notifier := &fakeNotifier{}
	s := &Service{Notifier: notifier, Logger: silentLogger()}
	s.NotifyRunFinished(context.Background(), testRun(domain.RunSkipped), "beresta", Preferences{NotifySuccess: true, NotifyFailure: true})

	if len(notifier.sent) != 0 {
		t.Errorf("sent %d notifications for a SKIPPED run, want 0", len(notifier.sent))
	}
}

func TestService_NotifyRunFinished_PreferencesFilter(t *testing.T) {
	t.Parallel()

	t.Run("success notifications disabled", func(t *testing.T) {
		t.Parallel()
		notifier := &fakeNotifier{}
		s := &Service{Notifier: notifier, Logger: silentLogger()}
		s.NotifyRunFinished(context.Background(), testRun(domain.RunSuccess), "beresta", Preferences{NotifySuccess: false, NotifyFailure: true})
		if len(notifier.sent) != 0 {
			t.Error("expected no notification when success notifications are disabled")
		}
	})

	t.Run("failure notifications still fire when disabled independently", func(t *testing.T) {
		t.Parallel()
		notifier := &fakeNotifier{}
		s := &Service{Notifier: notifier, Logger: silentLogger()}
		s.NotifyRunFinished(context.Background(), testRun(domain.RunFailed), "beresta", Preferences{NotifySuccess: false, NotifyFailure: true})
		if len(notifier.sent) != 1 {
			t.Error("expected a failure notification even though success notifications are disabled")
		}
	})

	t.Run("failure notifications disabled", func(t *testing.T) {
		t.Parallel()
		notifier := &fakeNotifier{}
		s := &Service{Notifier: notifier, Logger: silentLogger()}
		s.NotifyRunFinished(context.Background(), testRun(domain.RunFailed), "beresta", Preferences{NotifySuccess: true, NotifyFailure: false})
		if len(notifier.sent) != 0 {
			t.Error("expected no notification when failure notifications are disabled")
		}
	})
}

func TestService_NotifyRunFinished_RedactsSecrets(t *testing.T) {
	t.Parallel()
	notifier := &fakeNotifier{}
	redact := func(s string) string {
		return strings.ReplaceAll(s, "hunter2", "***")
	}
	s := &Service{Notifier: notifier, Redact: redact, Logger: silentLogger()}

	run := testRun(domain.RunFailed)
	run.ErrorMessage = "authentication failed with passphrase hunter2"
	s.NotifyRunFinished(context.Background(), run, "beresta", Preferences{NotifySuccess: true, NotifyFailure: true})

	if len(notifier.sent) != 1 {
		t.Fatalf("sent %d notifications, want 1", len(notifier.sent))
	}
	if strings.Contains(notifier.sent[0].Body, "hunter2") {
		t.Errorf("Body = %q, must not contain the secret", notifier.sent[0].Body)
	}
}

func TestService_NotifyRunFinished_DeliveryFailureDoesNotAffectRun(t *testing.T) {
	t.Parallel()
	notifier := &fakeNotifier{err: errors.New("notification subsystem unavailable")}
	s := &Service{Notifier: notifier, Logger: silentLogger()}

	run := testRun(domain.RunSuccess)
	beforeStatus := run.Status
	s.NotifyRunFinished(context.Background(), run, "beresta", Preferences{NotifySuccess: true, NotifyFailure: true})

	if run.Status != beforeStatus {
		t.Errorf("run status changed from %q to %q after a notification delivery failure", beforeStatus, run.Status)
	}
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(discardWriter{}, nil))
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
