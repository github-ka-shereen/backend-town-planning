package utils

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type DateOnly time.Time

func (d *DateOnly) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return err
	}
	*d = DateOnly(t)
	return nil
}

func (d DateOnly) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(d).Format("2006-01-02"))
}

// Value implements the driver.Valuer interface for database writes
func (d DateOnly) Value() (driver.Value, error) {
	return time.Time(d).Format("2006-01-02"), nil
}

// Scan implements the sql.Scanner interface for database reads
func (d *DateOnly) Scan(value interface{}) error {
	if value == nil {
		*d = DateOnly(time.Time{})
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		*d = DateOnly(v)
		return nil
	case string:
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return err
		}
		*d = DateOnly(t)
		return nil
	default:
		return fmt.Errorf("cannot scan type %T into DateOnly", value)
	}
}
