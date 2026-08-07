package system

import (
	"errors"

	"jproxy-go/internal/store/sqlite"
)

func maskedRows(rows []sqlite.SystemConfig) []sqlite.SystemConfig {
	masked := make([]sqlite.SystemConfig, len(rows))
	for index, row := range rows {
		masked[index] = row
		if isSensitiveConfigKey(row.Key) && row.Value != nil {
			value := maskedSecret
			masked[index].Value = &value
		}
	}
	return masked
}

func restoreMaskedSecrets(rows, current []sqlite.SystemConfig) error {
	values := make(map[string]string, len(current))
	for _, row := range current {
		if row.Value != nil {
			values[row.Key] = *row.Value
		}
	}
	for index := range rows {
		if !isSensitiveConfigKey(rows[index].Key) || rows[index].Value == nil || *rows[index].Value != maskedSecret {
			continue
		}
		value, ok := values[rows[index].Key]
		if !ok {
			return errors.New("missing current config value")
		}
		rows[index].Value = &value
	}
	return nil
}
