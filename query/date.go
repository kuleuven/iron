package query

import (
	"fmt"
	"strconv"
	"time"
)

func ParseTime(timestring string) (time.Time, error) {
	i64, err := strconv.ParseInt(timestring, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot parse IRODS time string '%s'", timestring)
	}

	if i64 <= 0 {
		return time.Time{}, nil
	}

	return time.Unix(i64, 0), nil
}
