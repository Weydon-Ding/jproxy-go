package sqlite

import (
	"errors"
	"fmt"
)

const batchLimit = 200

var ErrBatchTooLarge = errors.New("SQLite repository batch exceeds 200 rows")
var ErrInvalidValidStatus = errors.New("invalid valid status")
var ErrInvalidMonitoredStatus = errors.New("invalid monitored status")
var ErrJavaIntegerRange = errors.New("value exceeds Java Integer range")

type ValidStatus int64

const (
	Invalid ValidStatus = 0
	Valid   ValidStatus = 1
)

type MonitoredStatus int64

const (
	Unmonitored MonitoredStatus = 0
	Monitored   MonitoredStatus = 1
)

func ParseValidStatus(raw int64) (ValidStatus, error) {
	if raw != 0 && raw != 1 {
		return 0, fmt.Errorf("%d: %w", raw, ErrInvalidValidStatus)
	}
	return ValidStatus(raw), nil
}
func ParseMonitoredStatus(raw int64) (MonitoredStatus, error) {
	if raw != 0 && raw != 1 {
		return 0, fmt.Errorf("%d: %w", raw, ErrInvalidMonitoredStatus)
	}
	return MonitoredStatus(raw), nil
}
func validStatus(status ValidStatus) error { _, err := ParseValidStatus(int64(status)); return err }
func monitoredStatus(status MonitoredStatus) error {
	_, err := ParseMonitoredStatus(int64(status))
	return err
}

type SystemConfigID int64
type SystemUserID int64
type SonarrTitleID int64
type RadarrTitleID int64
type TMDBTitleID int64
type RuleID string
type SonarrRuleInput struct {
	Rule        SonarrRule
	ValidStatus *ValidStatus
}
type RadarrRuleInput struct {
	Rule        RadarrRule
	ValidStatus *ValidStatus
}
type RadarrTitleBatch struct{ Rows []RadarrTitle }
type RadarrTitleIDs struct{ IDs []RadarrTitleID }
type SonarrTitleBatch struct{ Rows []SonarrTitle }
type SonarrTitleIDs struct{ IDs []SonarrTitleID }
type TMDBTitleBatch struct{ Rows []TMDBTitle }
type TMDBTitleIDs struct{ IDs []TMDBTitleID }
type SonarrRuleBatch struct{ Rows []SonarrRule }
type RadarrRuleBatch struct{ Rows []RadarrRule }
type RuleIDs struct{ IDs []RuleID }

func javaInteger(value int64) error {
	if value < -2147483648 || value > 2147483647 {
		return fmt.Errorf("%d: %w", value, ErrJavaIntegerRange)
	}
	return nil
}

func javaIntegerPointer(value *int64) error {
	if value == nil {
		return nil
	}
	return javaInteger(*value)
}
