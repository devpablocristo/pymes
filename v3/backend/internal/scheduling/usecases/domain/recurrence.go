package domain

import (
	"sort"
	"time"
)

// ExpandRecurrence materializes local wall-clock occurrences and then converts
// each occurrence to an instant. That preserves the intended local time across
// daylight-saving transitions instead of adding fixed 24-hour durations.
func ExpandRecurrence(startAt time.Time, timezone string, rule RecurrenceRule) ([]time.Time, error) {
	if err := rule.Validate(); err != nil {
		return nil, err
	}
	location, err := EnsureTimezone(timezone)
	if err != nil {
		return nil, NewError(CodeValidation, "recurrence timezone is invalid")
	}
	local := startAt.In(location)
	hour, minute, second := local.Clock()
	nanosecond := local.Nanosecond()
	startDate := time.Date(local.Year(), local.Month(), local.Day(), hour, minute, second, nanosecond, location)
	limit := rule.Count
	if limit == 0 {
		limit = 500
	}
	until := time.Time{}
	if rule.Until != nil {
		until = rule.Until.UTC()
	}

	result := make([]time.Time, 0, limit)
	switch rule.Frequency {
	case RecurrenceDaily:
		for index := 0; len(result) < limit; index++ {
			candidateDate := startDate.AddDate(0, 0, index*rule.Interval)
			candidate := time.Date(candidateDate.Year(), candidateDate.Month(), candidateDate.Day(), hour, minute, second, nanosecond, location)
			if !until.IsZero() && candidate.UTC().After(until) {
				break
			}
			result = append(result, candidate.UTC())
		}
	case RecurrenceWeekly:
		weekdays := append([]time.Weekday(nil), rule.ByWeekdays...)
		if len(weekdays) == 0 {
			weekdays = []time.Weekday{startDate.Weekday()}
		}
		sort.Slice(weekdays, func(i, j int) bool { return weekdays[i] < weekdays[j] })
		unique := weekdays[:0]
		seen := make(map[time.Weekday]struct{}, len(weekdays))
		for _, weekday := range weekdays {
			if weekday < time.Sunday || weekday > time.Saturday {
				return nil, NewError(CodeValidation, "recurrence weekday is invalid")
			}
			if _, duplicate := seen[weekday]; duplicate {
				continue
			}
			seen[weekday] = struct{}{}
			unique = append(unique, weekday)
		}
		weekdays = unique
		for week := 0; len(result) < limit; week++ {
			weekStart := startDate.AddDate(0, 0, week*rule.Interval*7-int(startDate.Weekday()))
			for _, weekday := range weekdays {
				date := weekStart.AddDate(0, 0, int(weekday))
				candidate := time.Date(date.Year(), date.Month(), date.Day(), hour, minute, second, nanosecond, location)
				if candidate.Before(startDate) {
					continue
				}
				if !until.IsZero() && candidate.UTC().After(until) {
					return result, nil
				}
				result = append(result, candidate.UTC())
				if len(result) == limit {
					break
				}
			}
		}
	}
	if len(result) == 0 {
		return nil, NewError(CodeValidation, "recurrence produces no occurrences")
	}
	return result, nil
}
