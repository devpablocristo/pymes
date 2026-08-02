package helpers

import (
	"sort"
	"time"

	models "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/platform_scheduling/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/google/uuid"
)

func WindowsFor(
	day time.Time,
	resourceID uuid.UUID,
	rules []domain.AvailabilityRule,
	exceptions []domain.AvailabilityException,
	location *time.Location,
) []models.LocalWindow {
	resourceRules := make([]domain.AvailabilityRule, 0)
	branchRules := make([]domain.AvailabilityRule, 0)
	for _, rule := range rules {
		if !rule.Active || rule.Weekday != day.Weekday() || !dateApplies(day, rule, location) {
			continue
		}
		if rule.ResourceID != nil && *rule.ResourceID == resourceID {
			resourceRules = append(resourceRules, rule)
		} else if rule.Kind == domain.AvailabilityBranch {
			branchRules = append(branchRules, rule)
		}
	}
	branchWindows := ruleWindows(day, branchRules, location)
	resourceWindows := ruleWindows(day, resourceRules, location)
	windows := branchWindows
	if len(resourceWindows) > 0 {
		windows = intersectWindows(branchWindows, resourceWindows)
	}
	for _, exception := range exceptions {
		if exception.Kind != domain.ExceptionAvailability ||
			(exception.ResourceID != nil && *exception.ResourceID != resourceID) ||
			!exception.EndAt.After(exception.StartAt) {
			continue
		}
		windows = append(windows, models.LocalWindow{StartAt: exception.StartAt, EndAt: exception.EndAt})
	}
	sort.Slice(windows, func(i, j int) bool { return windows[i].StartAt.Before(windows[j].StartAt) })
	return windows
}

func ruleWindows(day time.Time, rules []domain.AvailabilityRule, location *time.Location) []models.LocalWindow {
	windows := make([]models.LocalWindow, 0, len(rules))
	for _, rule := range rules {
		startAt := wallMinute(day, rule.StartMinute, location)
		endMinute := rule.EndMinute
		if endMinute <= rule.StartMinute {
			endMinute += 1440
		}
		endAt := wallMinute(day, endMinute, location)
		if endAt.After(startAt) {
			windows = append(windows, models.LocalWindow{StartAt: startAt, EndAt: endAt})
		}
	}
	return windows
}

func intersectWindows(left, right []models.LocalWindow) []models.LocalWindow {
	result := make([]models.LocalWindow, 0)
	for _, first := range left {
		for _, second := range right {
			startAt := first.StartAt
			if second.StartAt.After(startAt) {
				startAt = second.StartAt
			}
			endAt := first.EndAt
			if second.EndAt.Before(endAt) {
				endAt = second.EndAt
			}
			if endAt.After(startAt) {
				result = append(result, models.LocalWindow{StartAt: startAt, EndAt: endAt})
			}
		}
	}
	return result
}

func Blocked(
	startAt, endAt time.Time,
	resourceID uuid.UUID,
	exceptions []domain.AvailabilityException,
) bool {
	for _, exception := range exceptions {
		if !exception.Blocking() {
			continue
		}
		if exception.ResourceID != nil && *exception.ResourceID != resourceID {
			continue
		}
		if startAt.Before(exception.EndAt) && endAt.After(exception.StartAt) {
			return true
		}
	}
	return false
}

func wallMinute(day time.Time, minute int, location *time.Location) time.Time {
	date := day.In(location)
	dayOffset := minute / 1440
	minute %= 1440
	base := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, location).AddDate(0, 0, dayOffset)
	return time.Date(base.Year(), base.Month(), base.Day(), minute/60, minute%60, 0, 0, location)
}

func dateApplies(day time.Time, rule domain.AvailabilityRule, location *time.Location) bool {
	date := time.Date(day.In(location).Year(), day.In(location).Month(), day.In(location).Day(), 0, 0, 0, 0, location)
	if rule.ValidFrom != nil {
		from := rule.ValidFrom.In(location)
		fromDate := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, location)
		if date.Before(fromDate) {
			return false
		}
	}
	if rule.ValidUntil != nil {
		until := rule.ValidUntil.In(location)
		untilDate := time.Date(until.Year(), until.Month(), until.Day(), 23, 59, 59, 0, location)
		if date.After(untilDate) {
			return false
		}
	}
	return true
}
