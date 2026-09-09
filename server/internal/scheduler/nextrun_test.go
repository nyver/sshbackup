package scheduler

import (
	"testing"
	"time"

	"vpsbackupmanager/internal/domain"
)

func mustParse(t *testing.T, layout, value string) time.Time {
	t.Helper()
	tm, err := time.Parse(layout, value)
	if err != nil {
		t.Fatalf("parse %q: %v", value, err)
	}
	return tm
}

func TestNextRun_Manual(t *testing.T) {
	t.Parallel()
	after := mustParse(t, time.RFC3339, "2026-09-08T00:00:00Z")
	_, ok, err := NextRun(domain.Schedule{Type: domain.ScheduleManual}, after)
	if err != nil {
		t.Fatalf("NextRun() error = %v", err)
	}
	if ok {
		t.Error("expected ok = false for a manual schedule")
	}
}

func TestNextRun_Daily(t *testing.T) {
	t.Parallel()
	sched := domain.Schedule{Type: domain.ScheduleDaily, Hour: 3, Minute: 0}

	tests := []struct {
		name  string
		after string
		want  string
	}{
		{"before today's occurrence", "2026-09-08T00:00:00Z", "2026-09-08T03:00:00Z"},
		{"after today's occurrence rolls to tomorrow", "2026-09-08T03:00:00Z", "2026-09-09T03:00:00Z"},
		{"mid-afternoon rolls to tomorrow", "2026-09-08T15:30:00Z", "2026-09-09T03:00:00Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			after := mustParse(t, time.RFC3339, tt.after)
			got, ok, err := NextRun(sched, after)
			if err != nil {
				t.Fatalf("NextRun() error = %v", err)
			}
			if !ok {
				t.Fatal("expected ok = true for a daily schedule")
			}
			want := mustParse(t, time.RFC3339, tt.want)
			if !got.Equal(want) {
				t.Errorf("NextRun() = %v, want %v", got, want)
			}
		})
	}
}

func TestNextRun_Weekly(t *testing.T) {
	t.Parallel()
	// Monday, Wednesday, Friday at 03:00.
	sched := domain.Schedule{
		Type: domain.ScheduleWeekly, Hour: 3, Minute: 0,
		Weekdays: []time.Weekday{time.Monday, time.Wednesday, time.Friday},
	}
	// 2026-09-08 is a Tuesday.
	after := mustParse(t, time.RFC3339, "2026-09-08T00:00:00Z")
	got, ok, err := NextRun(sched, after)
	if err != nil {
		t.Fatalf("NextRun() error = %v", err)
	}
	if !ok {
		t.Fatal("expected ok = true for a weekly schedule")
	}
	want := mustParse(t, time.RFC3339, "2026-09-09T03:00:00Z") // the next Wednesday
	if !got.Equal(want) {
		t.Errorf("NextRun() = %v, want %v (earliest of Mon/Wed/Fri)", got, want)
	}
}

func TestNextRun_Monthly_DayShorterThanMonth(t *testing.T) {
	t.Parallel()
	sched := domain.Schedule{Type: domain.ScheduleMonthly, DayOfMonth: 31, Hour: 3, Minute: 0}
	// September has 30 days.
	after := mustParse(t, time.RFC3339, "2026-09-01T00:00:00Z")
	got, ok, err := NextRun(sched, after)
	if err != nil {
		t.Fatalf("NextRun() error = %v", err)
	}
	if !ok {
		t.Fatal("expected ok = true for a monthly schedule")
	}
	want := mustParse(t, time.RFC3339, "2026-09-30T03:00:00Z")
	if !got.Equal(want) {
		t.Errorf("NextRun() = %v, want %v (clamped to the last day of September)", got, want)
	}
}

func TestNextRun_Monthly_RollsToNextMonth(t *testing.T) {
	t.Parallel()
	sched := domain.Schedule{Type: domain.ScheduleMonthly, DayOfMonth: 15, Hour: 3, Minute: 0}
	after := mustParse(t, time.RFC3339, "2026-09-20T00:00:00Z")
	got, ok, err := NextRun(sched, after)
	if err != nil {
		t.Fatalf("NextRun() error = %v", err)
	}
	if !ok {
		t.Fatal("expected ok = true")
	}
	want := mustParse(t, time.RFC3339, "2026-10-15T03:00:00Z")
	if !got.Equal(want) {
		t.Errorf("NextRun() = %v, want %v", got, want)
	}
}

func TestNextRun_Cron(t *testing.T) {
	t.Parallel()
	sched := domain.Schedule{Type: domain.ScheduleCron, CronExpression: "*/15 * * * *"}
	after := mustParse(t, time.RFC3339, "2026-09-08T00:05:00Z")
	got, ok, err := NextRun(sched, after)
	if err != nil {
		t.Fatalf("NextRun() error = %v", err)
	}
	if !ok {
		t.Fatal("expected ok = true")
	}
	want := mustParse(t, time.RFC3339, "2026-09-08T00:15:00Z")
	if !got.Equal(want) {
		t.Errorf("NextRun() = %v, want %v", got, want)
	}
}

func TestNextRun_InvalidCronExpression(t *testing.T) {
	t.Parallel()
	sched := domain.Schedule{Type: domain.ScheduleCron, CronExpression: "not a cron expression"}
	_, _, err := NextRun(sched, time.Now())
	if err == nil {
		t.Fatal("expected an error for an invalid cron expression")
	}
}
