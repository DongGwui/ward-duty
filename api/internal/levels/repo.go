package levels

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct{ PG *pgxpool.Pool }

func NewRepo(pg *pgxpool.Pool) *Repo { return &Repo{PG: pg} }

func (r *Repo) List(ctx context.Context) ([]Level, error) {
	rows, err := r.PG.Query(ctx, `
		SELECT id, code, display_name, min_months, max_months,
		       min_d, min_e, min_n,
		       weight_coverage, weight_d_assignment, weight_e_assignment, weight_n_assignment,
		       sort_order, updated_at
		FROM experience_levels
		ORDER BY sort_order, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Level
	for rows.Next() {
		var l Level
		if err := rows.Scan(&l.ID, &l.Code, &l.DisplayName, &l.MinMonths, &l.MaxMonths,
			&l.MinD, &l.MinE, &l.MinN,
			&l.WeightCoverage, &l.WeightDAssignment, &l.WeightEAssignment, &l.WeightNAssignment,
			&l.SortOrder, &l.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *Repo) Get(ctx context.Context, code string) (*Level, error) {
	row := r.PG.QueryRow(ctx, `
		SELECT id, code, display_name, min_months, max_months,
		       min_d, min_e, min_n,
		       weight_coverage, weight_d_assignment, weight_e_assignment, weight_n_assignment,
		       sort_order, updated_at
		FROM experience_levels WHERE code = $1`, code)
	var l Level
	if err := row.Scan(&l.ID, &l.Code, &l.DisplayName, &l.MinMonths, &l.MaxMonths,
		&l.MinD, &l.MinE, &l.MinN,
		&l.WeightCoverage, &l.WeightDAssignment, &l.WeightEAssignment, &l.WeightNAssignment,
		&l.SortOrder, &l.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &l, nil
}

func (r *Repo) Create(ctx context.Context, in CreateInput) (*Level, error) {
	row := r.PG.QueryRow(ctx, `
		INSERT INTO experience_levels (
			code, display_name, min_months, max_months,
			min_d, min_e, min_n,
			weight_coverage, weight_d_assignment, weight_e_assignment, weight_n_assignment,
			sort_order
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, code, display_name, min_months, max_months,
		          min_d, min_e, min_n,
		          weight_coverage, weight_d_assignment, weight_e_assignment, weight_n_assignment,
		          sort_order, updated_at`,
		in.Code, in.DisplayName, in.MinMonths, in.MaxMonths,
		in.MinD, in.MinE, in.MinN,
		in.WeightCoverage, in.WeightDAssignment, in.WeightEAssignment, in.WeightNAssignment,
		in.SortOrder,
	)
	var l Level
	if err := row.Scan(&l.ID, &l.Code, &l.DisplayName, &l.MinMonths, &l.MaxMonths,
		&l.MinD, &l.MinE, &l.MinN,
		&l.WeightCoverage, &l.WeightDAssignment, &l.WeightEAssignment, &l.WeightNAssignment,
		&l.SortOrder, &l.UpdatedAt); err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *Repo) Update(ctx context.Context, code string, in UpdateInput) (*Level, error) {
	// 동적 SET 빌드
	sets := []string{}
	args := []any{}
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, col+" = $"+itoa(len(args)))
	}
	if in.DisplayName != nil {
		add("display_name", *in.DisplayName)
	}
	if in.MinMonths != nil {
		add("min_months", *in.MinMonths)
	}
	if in.MaxMonths != nil {
		add("max_months", *in.MaxMonths)
	}
	if in.MinD != nil {
		add("min_d", *in.MinD)
	}
	if in.MinE != nil {
		add("min_e", *in.MinE)
	}
	if in.MinN != nil {
		add("min_n", *in.MinN)
	}
	if in.WeightCoverage != nil {
		add("weight_coverage", *in.WeightCoverage)
	}
	if in.WeightDAssignment != nil {
		add("weight_d_assignment", *in.WeightDAssignment)
	}
	if in.WeightEAssignment != nil {
		add("weight_e_assignment", *in.WeightEAssignment)
	}
	if in.WeightNAssignment != nil {
		add("weight_n_assignment", *in.WeightNAssignment)
	}
	if in.SortOrder != nil {
		add("sort_order", *in.SortOrder)
	}
	if len(sets) == 0 {
		return r.Get(ctx, code)
	}
	args = append(args, code)
	q := "UPDATE experience_levels SET " + joinCSV(sets) + ", updated_at = NOW() WHERE code = $" + itoa(len(args))
	if _, err := r.PG.Exec(ctx, q, args...); err != nil {
		return nil, err
	}
	return r.Get(ctx, code)
}

func (r *Repo) Delete(ctx context.Context, code string) error {
	// 소속 nurse 0명이어야 삭제 가능
	var cnt int
	if err := r.PG.QueryRow(ctx,
		`SELECT COUNT(*) FROM nurses WHERE experience_level_override = $1`, code).Scan(&cnt); err != nil {
		return err
	}
	if cnt > 0 {
		return ErrLevelInUse
	}
	res, err := r.PG.Exec(ctx, `DELETE FROM experience_levels WHERE code = $1`, code)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ----- errors -----

var (
	ErrNotFound   = errors.New("level not found")
	ErrLevelInUse = errors.New("level is referenced by nurse override")
)

// ----- helpers -----

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

func joinCSV(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
