package titlesync

import (
	"errors"
	"strconv"
)

var errInvalidRows = errors.New("invalid title rows")

func stableID(externalID, sno int64) (int64, error) {
	if externalID <= 0 || sno < 0 {
		return 0, errInvalidRows
	}
	value, err := strconv.ParseInt(strconv.FormatInt(externalID, 10)+strconv.FormatInt(sno, 10), 10, 32)
	if err != nil {
		return 0, errInvalidRows
	}
	return value, nil
}
