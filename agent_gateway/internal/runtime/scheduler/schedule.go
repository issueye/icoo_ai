package scheduler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func NextRun(schedule Schedule, after time.Time) (*time.Time, error) {
	loc := after.Location()
	if schedule.Timezone != "" {
		loaded, err := time.LoadLocation(schedule.Timezone)
		if err != nil {
			return nil, err
		}
		loc = loaded
	}
	after = after.In(loc)

	switch schedule.Kind {
	case ScheduleAt:
		if schedule.At == nil {
			return nil, errors.New("at schedule missing time")
		}
		at := schedule.At.In(loc)
		if at.After(after) {
			return &at, nil
		}
		return nil, nil
	case ScheduleEvery:
		if schedule.Every <= 0 {
			return nil, errors.New("every schedule must be positive")
		}
		next := after.Add(schedule.Every)
		return &next, nil
	case ScheduleCron:
		return nextCron(schedule.CronExpr, after)
	default:
		return nil, fmt.Errorf("unknown schedule kind %q", schedule.Kind)
	}
}

func nextCron(expr string, after time.Time) (*time.Time, error) {
	spec, err := parseCron(expr)
	if err != nil {
		return nil, err
	}
	cursor := after.Truncate(time.Minute).Add(time.Minute)
	limit := cursor.AddDate(2, 0, 0)
	for cursor.Before(limit) {
		if spec.matches(cursor) {
			next := cursor
			return &next, nil
		}
		cursor = cursor.Add(time.Minute)
	}
	return nil, errors.New("cron next run not found within two years")
}

type cronSpec struct {
	minute     fieldMatcher
	hour       fieldMatcher
	dayOfMonth fieldMatcher
	month      fieldMatcher
	weekday    fieldMatcher
}

func (s cronSpec) matches(t time.Time) bool {
	return s.minute.match(t.Minute()) &&
		s.hour.match(t.Hour()) &&
		s.dayOfMonth.match(t.Day()) &&
		s.month.match(int(t.Month())) &&
		s.weekday.match(int(t.Weekday()))
}

type fieldMatcher struct {
	any    bool
	values map[int]struct{}
}

func (m fieldMatcher) match(value int) bool {
	if m.any {
		return true
	}
	_, ok := m.values[value]
	return ok
}

func parseCron(expr string) (cronSpec, error) {
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return cronSpec{}, errors.New("cron expression must have 5 fields")
	}
	minute, err := parseCronField(parts[0], 0, 59)
	if err != nil {
		return cronSpec{}, fmt.Errorf("minute: %w", err)
	}
	hour, err := parseCronField(parts[1], 0, 23)
	if err != nil {
		return cronSpec{}, fmt.Errorf("hour: %w", err)
	}
	dayOfMonth, err := parseCronField(parts[2], 1, 31)
	if err != nil {
		return cronSpec{}, fmt.Errorf("day of month: %w", err)
	}
	month, err := parseCronField(parts[3], 1, 12)
	if err != nil {
		return cronSpec{}, fmt.Errorf("month: %w", err)
	}
	weekday, err := parseCronField(parts[4], 0, 6)
	if err != nil {
		return cronSpec{}, fmt.Errorf("weekday: %w", err)
	}
	return cronSpec{minute: minute, hour: hour, dayOfMonth: dayOfMonth, month: month, weekday: weekday}, nil
}

func parseCronField(raw string, minValue, maxValue int) (fieldMatcher, error) {
	if raw == "*" {
		return fieldMatcher{any: true}, nil
	}
	matcher := fieldMatcher{values: map[int]struct{}{}}
	for _, token := range strings.Split(raw, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			return fieldMatcher{}, errors.New("empty token")
		}
		step := 1
		base := token
		if strings.Contains(token, "/") {
			var stepRaw string
			base, stepRaw, _ = strings.Cut(token, "/")
			parsed, err := strconv.Atoi(stepRaw)
			if err != nil || parsed <= 0 {
				return fieldMatcher{}, errors.New("invalid step")
			}
			step = parsed
		}
		var start, end int
		switch {
		case base == "*":
			start, end = minValue, maxValue
		case strings.Contains(base, "-"):
			left, right, _ := strings.Cut(base, "-")
			var err error
			start, err = strconv.Atoi(left)
			if err != nil {
				return fieldMatcher{}, err
			}
			end, err = strconv.Atoi(right)
			if err != nil {
				return fieldMatcher{}, err
			}
		default:
			parsed, err := strconv.Atoi(base)
			if err != nil {
				return fieldMatcher{}, err
			}
			start, end = parsed, parsed
		}
		if start < minValue || end > maxValue || start > end {
			return fieldMatcher{}, errors.New("value out of range")
		}
		for value := start; value <= end; value += step {
			matcher.values[value] = struct{}{}
		}
	}
	return matcher, nil
}
