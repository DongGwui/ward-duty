// Package dateonly — DB DATE 컬럼을 위한 wrapper 타입.
//
// 문제: Go의 time.Time을 그대로 JSON 직렬화하면 "2026-07-15T00:00:00Z" (RFC3339).
// UI/Pydantic은 "2026-07-15"를 기대하므로 lookup miss + Pydantic 422 위험.
//
// 해결: pgx Scan/Value는 time.Time과 호환, JSON Marshal/Unmarshal은 "YYYY-MM-DD".
package dateonly

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

type Date time.Time

const layout = "2006-01-02"

func From(t time.Time) Date { return Date(t) }
func (d Date) Time() time.Time { return time.Time(d) }
func (d Date) String() string  { return time.Time(d).Format(layout) }

// ----- JSON -----

func (d Date) MarshalJSON() ([]byte, error) {
	t := time.Time(d)
	if t.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + t.Format(layout) + `"`), nil
}

func (d *Date) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*d = Date(time.Time{})
		return nil
	}
	// YYYY-MM-DD 우선
	if t, err := time.Parse(layout, s); err == nil {
		*d = Date(t)
		return nil
	}
	// RFC3339 fallback (구 클라이언트 호환)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		*d = Date(t)
		return nil
	}
	return fmt.Errorf("invalid date: %s", s)
}

// ----- pgx Scanner / Valuer -----

func (d *Date) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*d = Date(time.Time{})
		return nil
	case time.Time:
		*d = Date(v)
		return nil
	case string:
		t, err := time.Parse(layout, v)
		if err != nil {
			return err
		}
		*d = Date(t)
		return nil
	case []byte:
		t, err := time.Parse(layout, string(v))
		if err != nil {
			return err
		}
		*d = Date(t)
		return nil
	default:
		return fmt.Errorf("cannot scan %T into dateonly.Date", src)
	}
}

func (d Date) Value() (driver.Value, error) {
	t := time.Time(d)
	if t.IsZero() {
		return nil, nil
	}
	return t, nil
}
