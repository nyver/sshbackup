package domain

import (
	"testing"
	"time"
)

func validJobParams() NewJobParams {
	return NewJobParams{
		Name:             "beresta",
		ServerID:         "server-1",
		Enabled:          true,
		Sources:          []Source{{RemotePath: "/docker/volumes"}},
		Schedule:         Schedule{Type: ScheduleManual},
		LocalDestination: `D:\Backups\beresta`,
	}
}

func TestNewJob(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 8, 3, 0, 0, 0, time.UTC)

	t.Run("valid job gets defaults", func(t *testing.T) {
		t.Parallel()
		j, err := NewJob(now, validJobParams())
		if err != nil {
			t.Fatalf("NewJob() error = %v", err)
		}
		if j.RemoteTempDirectory != DefaultRemoteTempDirectory {
			t.Errorf("expected default remote temp dir, got %q", j.RemoteTempDirectory)
		}
		if j.ArchiveTimeout != DefaultArchiveTimeout {
			t.Errorf("expected default archive timeout, got %v", j.ArchiveTimeout)
		}
		if j.Sources[0].JobID != j.ID {
			t.Error("expected source to be stamped with the job id")
		}
		if j.Schedule.MissedRunPolicy != MissedRunAsSoonAsPossible {
			t.Errorf("expected default missed run policy, got %q", j.Schedule.MissedRunPolicy)
		}
	})

	t.Run("no sources is rejected", func(t *testing.T) {
		t.Parallel()
		p := validJobParams()
		p.Sources = nil
		if _, err := NewJob(now, p); err == nil {
			t.Fatal("expected an error for a job with no sources")
		}
	})

	t.Run("relative source path is rejected", func(t *testing.T) {
		t.Parallel()
		p := validJobParams()
		p.Sources = []Source{{RemotePath: "docker/volumes"}}
		if _, err := NewJob(now, p); err == nil {
			t.Fatal("expected an error for a relative source path")
		}
	})

	t.Run("empty name is rejected", func(t *testing.T) {
		t.Parallel()
		p := validJobParams()
		p.Name = ""
		if _, err := NewJob(now, p); err == nil {
			t.Fatal("expected an error for an empty job name")
		}
	})

	t.Run("script gets default timeout", func(t *testing.T) {
		t.Parallel()
		p := validJobParams()
		p.Scripts = []Script{{
			Type: ScriptPreBackup, Command: "docker compose down",
			RunCondition: RunConditionAlways,
		}}
		j, err := NewJob(now, p)
		if err != nil {
			t.Fatalf("NewJob() error = %v", err)
		}
		if j.Scripts[0].TimeoutSeconds != DefaultScriptTimeoutSeconds {
			t.Errorf("expected default script timeout, got %d", j.Scripts[0].TimeoutSeconds)
		}
	})
}

func TestArchiveFileName(t *testing.T) {
	t.Parallel()
	startedAt := time.Date(2026, 9, 8, 3, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		jobName string
		want    string
	}{
		{"simple name", "beresta", "beresta_2026-09-08_03-00-00.tar.gz"},
		{"unsafe characters replaced", `family:hub/prod`, "family_hub_prod_2026-09-08_03-00-00.tar.gz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ArchiveFileName(tt.jobName, startedAt)
			if got != tt.want {
				t.Errorf("ArchiveFileName(%q) = %q, want %q", tt.jobName, got, tt.want)
			}
		})
	}
}

func TestScheduleValidate_MonthlyDayOutOfRange(t *testing.T) {
	t.Parallel()
	s := &Schedule{Type: ScheduleMonthly, Hour: 3, DayOfMonth: 32, MissedRunPolicy: MissedRunSkip}
	if err := s.Validate(); err == nil {
		t.Fatal("expected an error for a day of month above 31")
	}
}

func TestScheduleValidate_WeeklyRequiresWeekdays(t *testing.T) {
	t.Parallel()
	s := &Schedule{Type: ScheduleWeekly, Hour: 3, MissedRunPolicy: MissedRunSkip}
	if err := s.Validate(); err == nil {
		t.Fatal("expected an error for a weekly schedule with no weekdays")
	}
}

func TestScriptShouldRun(t *testing.T) {
	t.Parallel()
	tests := []struct {
		condition RunCondition
		failed    bool
		want      bool
	}{
		{RunConditionAlways, true, true},
		{RunConditionAlways, false, true},
		{RunConditionOnFailure, true, true},
		{RunConditionOnFailure, false, false},
		{RunConditionOnSuccess, true, false},
		{RunConditionOnSuccess, false, true},
	}
	for _, tt := range tests {
		s := &Script{RunCondition: tt.condition}
		got := s.ShouldRun(tt.failed)
		if got != tt.want {
			t.Errorf("ShouldRun(%s, failed=%v) = %v, want %v", tt.condition, tt.failed, got, tt.want)
		}
	}
}
