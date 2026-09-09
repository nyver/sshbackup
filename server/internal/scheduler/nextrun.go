// Package scheduler computes next-run times for job schedules and
// dispatches due jobs under the global/per-server/per-job concurrency
// limits, honoring the global pause and missed-run policy.
package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"vpsbackupmanager/internal/domain"
)

// cronParser matches the standard five-field expression (minute hour
// day-of-month month day-of-week); robfig/cron numbers Sunday as 0,
// matching time.Weekday exactly, so weekly schedules need no translation.
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// NextRun computes the next occurrence of schedule strictly after after,
// in after's time zone (the caller passes host local time). ok is false
// for MANUAL schedules, which are never dispatched automatically.
func NextRun(schedule domain.Schedule, after time.Time) (next time.Time, ok bool, err error) {
	switch schedule.Type {
	case domain.ScheduleManual:
		return time.Time{}, false, nil
	case domain.ScheduleDaily:
		return nextFromCronSpec(fmt.Sprintf("%d %d * * *", schedule.Minute, schedule.Hour), after)
	case domain.ScheduleWeekly:
		days := make([]string, len(schedule.Weekdays))
		for i, w := range schedule.Weekdays {
			days[i] = strconv.Itoa(int(w))
		}
		return nextFromCronSpec(fmt.Sprintf("%d %d * * %s", schedule.Minute, schedule.Hour, strings.Join(days, ",")), after)
	case domain.ScheduleMonthly:
		return nextMonthly(schedule, after)
	case domain.ScheduleCron:
		return nextFromCronSpec(schedule.CronExpression, after)
	default:
		return time.Time{}, false, fmt.Errorf("unknown schedule type %q", schedule.Type)
	}
}

func nextFromCronSpec(spec string, after time.Time) (time.Time, bool, error) {
	sched, err := cronParser.Parse(spec)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("parse cron spec %q: %w", spec, err)
	}
	return sched.Next(after), true, nil
}

// nextMonthly finds the next occurrence of schedule.DayOfMonth at
// Hour:Minute after `after`, clamping to the last day of a shorter month
// (e.g. day 31 in a 30-day month runs on day 30).
func nextMonthly(schedule domain.Schedule, after time.Time) (time.Time, bool, error) {
	loc := after.Location()
	candidate := monthlyOccurrence(after.Year(), int(after.Month()), schedule.DayOfMonth, schedule.Hour, schedule.Minute, loc)
	if !candidate.After(after) {
		year, month := after.Year(), int(after.Month())+1
		if month > 12 {
			month = 1
			year++
		}
		candidate = monthlyOccurrence(year, month, schedule.DayOfMonth, schedule.Hour, schedule.Minute, loc)
	}
	return candidate, true, nil
}

func monthlyOccurrence(year, month, day, hour, minute int, loc *time.Location) time.Time {
	if last := lastDayOfMonth(year, month); day > last {
		day = last
	}
	return time.Date(year, time.Month(month), day, hour, minute, 0, 0, loc)
}

func lastDayOfMonth(year, month int) int {
	firstOfNext := time.Date(year, time.Month(month+1), 1, 0, 0, 0, 0, time.UTC)
	return firstOfNext.AddDate(0, 0, -1).Day()
}
