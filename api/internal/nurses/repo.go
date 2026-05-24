package nurses

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct{ PG *pgxpool.Pool }

func NewRepo(pg *pgxpool.Pool) *Repo { return &Repo{PG: pg} }

const selectCols = `id, name, role, email, google_sub, hire_date,
	experience_level_override, fixed_shift_pattern, active, last_login_at, created_at`

func scanNurse(row pgx.Row, n *Nurse) error {
	return row.Scan(&n.ID, &n.Name, &n.Role, &n.Email, &n.GoogleSub, &n.HireDate,
		&n.ExperienceLevelOverride, &n.FixedShiftPattern, &n.Active, &n.LastLoginAt, &n.CreatedAt)
}

func (r *Repo) List(ctx context.Context, includeInactive bool) ([]Nurse, error) {
	q := "SELECT " + selectCols + " FROM nurses"
	if !includeInactive {
		q += " WHERE active = TRUE"
	}
	q += " ORDER BY active DESC, name"
	rows, err := r.PG.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Nurse
	for rows.Next() {
		var n Nurse
		if err := scanNurse(rows, &n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *Repo) Get(ctx context.Context, id int) (*Nurse, error) {
	row := r.PG.QueryRow(ctx, "SELECT "+selectCols+" FROM nurses WHERE id = $1", id)
	var n Nurse
	if err := scanNurse(row, &n); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &n, nil
}

func (r *Repo) Create(ctx context.Context, in CreateInput) (*Nurse, error) {
	role := in.Role
	if role == "" {
		role = "nurse"
	}
	// Stage 1: email은 옵션. 빈 문자열도 NULL로 처리.
	var emailParam any
	if in.Email != nil {
		normalized := strings.ToLower(strings.TrimSpace(*in.Email))
		if normalized != "" {
			emailParam = normalized
		}
	}
	row := r.PG.QueryRow(ctx, `
		INSERT INTO nurses (name, email, role, hire_date, experience_level_override, fixed_shift_pattern, active)
		VALUES ($1, $2, $3, $4, $5, $6, TRUE)
		RETURNING `+selectCols,
		in.Name, emailParam, role,
		in.HireDate, in.ExperienceLevelOverride, in.FixedShiftPattern)
	var n Nurse
	if err := scanNurse(row, &n); err != nil {
		return nil, err
	}
	return &n, nil
}

func (r *Repo) Update(ctx context.Context, id int, in UpdateInput) (*Nurse, error) {
	sets := []string{}
	args := []any{}
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, col+" = $"+itoa(len(args)))
	}
	if in.Name != nil {
		add("name", *in.Name)
	}
	if in.Role != nil {
		add("role", *in.Role)
	}
	if in.HireDate != nil {
		add("hire_date", *in.HireDate)
	}
	if in.ExperienceLevelOverride != nil {
		add("experience_level_override", *in.ExperienceLevelOverride)
	}
	if in.FixedShiftPattern != nil {
		// "" 빈 문자열을 NULL로 처리
		if *in.FixedShiftPattern == "" {
			add("fixed_shift_pattern", nil)
		} else {
			add("fixed_shift_pattern", *in.FixedShiftPattern)
		}
	}
	if in.Active != nil {
		add("active", *in.Active)
	}
	if len(sets) == 0 {
		return r.Get(ctx, id)
	}
	args = append(args, id)
	q := "UPDATE nurses SET " + strings.Join(sets, ", ") + " WHERE id = $" + itoa(len(args))
	if _, err := r.PG.Exec(ctx, q, args...); err != nil {
		return nil, err
	}
	return r.Get(ctx, id)
}

var ErrNotFound = errors.New("nurse not found")

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
