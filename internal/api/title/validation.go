package title

import "errors"

var errBatchTooLarge = errors.New("title batch exceeds 200 rows")

func validTMDBInput(input tmdbTitleDTO) bool {
	if input.ValidStatus != 0 && input.ValidStatus != 1 {
		return false
	}
	if _, err := parseIntegerInt64(input.TVDBID); err != nil {
		return false
	}
	if input.ID != nil {
		if _, err := parseIntegerInt64(*input.ID); err != nil {
			return false
		}
	}
	if input.TMDBID != nil {
		if _, err := parseIntegerInt64(*input.TMDBID); err != nil {
			return false
		}
	}
	return true
}

func parseIntegerInt64(value int64) (int64, error) {
	if value < -2147483648 || value > 2147483647 {
		return 0, errors.New("invalid integer")
	}
	return value, nil
}
