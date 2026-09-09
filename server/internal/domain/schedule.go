package domain

import (
	"errors"
	"fmt"
	"time"
)

// Schedule controls when a job's scheduler dispatches it. Only one of the
// type-specific fields is meaningful for a given Type; the others are
// ignored.
type Schedule struct {
	ID    string
	JobID string
	Type  ScheduleType

	// Hour and Minute apply to DAILY, WEEKLY, and MONTHLY schedules, in the
	// host's local time zone.
	Hour   int
	Minute int

	// Weekdays applies to WEEKLY schedules.
	Weekdays []time.Weekday

	// DayOfMonth applies to MONTHLY schedules. A day beyond the length of a
	// given month runs on that month's last day instead.
	DayOfMonth int

	// CronExpression applies to CRON schedules: a standard five-field
	// expression.
	CronExpression string

	MissedRunPolicy MissedRunPolicy
}

// Validate checks the invariants required by the scheduling specification
// for the schedule's declared Type.
func (s *Schedule) Validate() error {
	var errs []error

	switch s.Type {
	case ScheduleManual:
		// No further fields required.
	case ScheduleDaily:
		errs = append(errs, validateTimeOfDay(s.Hour, s.Minute))
	case ScheduleWeekly:
		errs = append(errs, validateTimeOfDay(s.Hour, s.Minute))
		if len(s.Weekdays) == 0 {
			errs = append(errs, errors.New("weekly schedule requires at least one weekday"))
		}
	case ScheduleMonthly:
		errs = append(errs, validateTimeOfDay(s.Hour, s.Minute))
		if s.DayOfMonth < 1 || s.DayOfMonth > 31 {
			errs = append(errs, fmt.Errorf("monthly schedule day %d must be between 1 and 31", s.DayOfMonth))
		}
	case ScheduleCron:
		errs = append(errs, ValidateCronExpression(s.CronExpression))
	default:
		errs = append(errs, fmt.Errorf("unknown schedule type %q", s.Type))
	}

	switch s.MissedRunPolicy {
	case MissedRunAsSoonAsPossible, MissedRunSkip:
	default:
		errs = append(errs, errors.New("missed run policy must be RUN_AS_SOON_AS_POSSIBLE or SKIP"))
	}

	return errors.Join(errs...)
}

func validateTimeOfDay(hour, minute int) error {
	if hour < 0 || hour > 23 {
		return fmt.Errorf("schedule hour %d must be between 0 and 23", hour)
	}
	if minute < 0 || minute > 59 {
		return fmt.Errorf("schedule minute %d must be between 0 and 59", minute)
	}
	return nil
}

// IsAutomatic reports whether the scheduler should ever dispatch this
// schedule on its own, as opposed to only via Run now.
func (s *Schedule) IsAutomatic() bool {
	return s.Type != ScheduleManual
}
