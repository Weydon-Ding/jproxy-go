package tasks

import (
	"errors"
	"fmt"
	"time"
)

// ErrValidation identifies invalid scheduler configuration.
var ErrValidation = errors.New("tasks: validation")

// ValidationError identifies the invalid scheduler field.
type ValidationError struct {
	Field string
	Rule  string
}

func (validation *ValidationError) Error() string {
	return fmt.Sprintf("tasks: %s must be %s", validation.Field, validation.Rule)
}

func (*ValidationError) Is(target error) bool { return target == ErrValidation }

// Schedule calculates the next wall-clock occurrence after a local time.
type Schedule interface {
	Next(time.Time) time.Time
	schedule()
}

type dailySchedule struct{ hour, minute, second int }
type hourlySchedule struct{ minute, second int }
type minuteSchedule struct{ interval, second int }
type secondSchedule struct{ interval int }

const scheduleSearchLimit = 72 * time.Hour

func (dailySchedule) schedule()  {}
func (hourlySchedule) schedule() {}
func (minuteSchedule) schedule() {}
func (secondSchedule) schedule() {}

// Daily returns a schedule at one local time each day.
func Daily(hour, minute, second int) (Schedule, error) {
	if err := validTime(hour, minute, second); err != nil {
		return nil, err
	}
	return dailySchedule{hour: hour, minute: minute, second: second}, nil
}

// Hourly returns a schedule at one minute and second each local hour.
func Hourly(minute, second int) (Schedule, error) {
	if err := validMinuteSecond(minute, second); err != nil {
		return nil, err
	}
	return hourlySchedule{minute: minute, second: second}, nil
}

// EveryMinutes returns a wall-clock schedule whose interval divides one hour.
func EveryMinutes(interval, second int) (Schedule, error) {
	if interval < 1 || interval > 60 || 60%interval != 0 {
		return nil, &ValidationError{Field: "interval", Rule: "a positive divisor of 60 minutes"}
	}
	if second < 0 || second > 59 {
		return nil, &ValidationError{Field: "second", Rule: "between 0 and 59"}
	}
	return minuteSchedule{interval: interval, second: second}, nil
}

// EverySeconds returns a wall-clock schedule whose interval divides one minute.
func EverySeconds(interval int) (Schedule, error) {
	if interval < 1 || interval > 60 || 60%interval != 0 {
		return nil, &ValidationError{Field: "interval", Rule: "a positive divisor of 60 seconds"}
	}
	return secondSchedule{interval: interval}, nil
}

func (schedule dailySchedule) Next(from time.Time) time.Time {
	return nextMatching(from, func(local time.Time) bool {
		return local.Hour() == schedule.hour && local.Minute() == schedule.minute && local.Second() == schedule.second
	})
}

func (schedule hourlySchedule) Next(from time.Time) time.Time {
	return nextMatching(from, func(local time.Time) bool {
		return local.Minute() == schedule.minute && local.Second() == schedule.second
	})
}

func (schedule minuteSchedule) Next(from time.Time) time.Time {
	return nextMatching(from, func(local time.Time) bool {
		return local.Minute()%schedule.interval == 0 && local.Second() == schedule.second
	})
}

func (schedule secondSchedule) Next(from time.Time) time.Time {
	return nextMatching(from, func(local time.Time) bool { return local.Second()%schedule.interval == 0 })
}

func nextMatching(from time.Time, matches func(time.Time) bool) time.Time {
	candidate := from.Truncate(time.Second).Add(time.Second)
	limit := from.Add(scheduleSearchLimit)
	for !candidate.After(limit) {
		if matches(candidate.In(from.Location())) {
			return candidate
		}
		candidate = candidate.Add(time.Second)
	}
	return candidate
}

func validTime(hour, minute, second int) error {
	if hour < 0 || hour > 23 {
		return &ValidationError{Field: "hour", Rule: "between 0 and 23"}
	}
	return validMinuteSecond(minute, second)
}

func validMinuteSecond(minute, second int) error {
	if minute < 0 || minute > 59 {
		return &ValidationError{Field: "minute", Rule: "between 0 and 59"}
	}
	if second < 0 || second > 59 {
		return &ValidationError{Field: "second", Rule: "between 0 and 59"}
	}
	return nil
}
