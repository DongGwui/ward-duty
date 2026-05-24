package nightkeepers

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct{ PG *pgxpool.Pool }

func NewRepo(pg *pgxpool.Pool) *Repo { return &Repo{PG: pg} }

const cols = `id, nurse_id, year_month, assigned_by_nurse_id, reason, created_at`

func (r *Repo) ListByMonth(ctx context.Context, ym string) ([]Assignment, error) {
	rows, err := r.PG.Query(ctx, "SELECT "+cols+" FROM night_keeper_assignments WHERE year_month = $1 ORDER BY nurse_id", ym)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Assignment
	for rows.Next() {
		var a Assignment
		if err := rows.Scan(&a.ID, &a.NurseID, &a.YearMonth, &a.AssignedByNurseID, &a.Reason, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// HasMonth — (nurse, year_month) 지정 존재?
func (r *Repo) HasMonth(ctx context.Context, nurseID int, ym string) (bool, error) {
	var x int
	err := r.PG.QueryRow(ctx,
		"SELECT 1 FROM night_keeper_assignments WHERE nurse_id = $1 AND year_month = $2", nurseID, ym).Scan(&x)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// FetchFixedPattern — K-05 검증용. nurse의 fixed_shift_pattern 반환 (NULL이면 nil).
func (r *Repo) FetchFixedPattern(ctx context.Context, nurseID int) (*string, error) {
	var p *string
	err := r.PG.QueryRow(ctx,
		"SELECT fixed_shift_pattern FROM nurses WHERE id = $1", nurseID).Scan(&p)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNurseNotFound
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *Repo) Create(ctx context.Context, in CreateInput, assignedBy int) (*Assignment, error) {
	row := r.PG.QueryRow(ctx, `
		INSERT INTO night_keeper_assignments (nurse_id, year_month, assigned_by_nurse_id, reason)
		VALUES ($1, $2, $3, $4)
		RETURNING `+cols,
		in.NurseID, in.YearMonth, assignedBy, in.Reason)
	var a Assignment
	if err := row.Scan(&a.ID, &a.NurseID, &a.YearMonth, &a.AssignedByNurseID, &a.Reason, &a.CreatedAt); err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *Repo) Delete(ctx context.Context, id int) error {
	res, err := r.PG.Exec(ctx, `DELETE FROM night_keeper_assignments WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

var (
	ErrNotFound       = errors.New("night_keeper_assignment not found")
	ErrNurseNotFound  = errors.New("nurse not found")
)
