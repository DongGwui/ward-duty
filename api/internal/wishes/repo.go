package wishes

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct{ PG *pgxpool.Pool }

func NewRepo(pg *pgxpool.Pool) *Repo { return &Repo{PG: pg} }

// ListByMonth — year_month (YYYY-MM) 범위. nurseFilter > 0이면 해당 nurse만.
func (r *Repo) ListByMonth(ctx context.Context, year, month int, nurseFilter int) ([]Wish, error) {
	from := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	q := `SELECT id, nurse_id, date, type, reason, created_at
	      FROM wishes WHERE date >= $1 AND date < $2`
	args := []any{from, to}
	if nurseFilter > 0 {
		q += " AND nurse_id = $3"
		args = append(args, nurseFilter)
	}
	q += " ORDER BY date, nurse_id"
	rows, err := r.PG.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Wish
	for rows.Next() {
		var w Wish
		if err := rows.Scan(&w.ID, &w.NurseID, &w.Date, &w.Type, &w.Reason, &w.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// Upsert — 본인 (nurse_id, date) upsert.
func (r *Repo) Upsert(ctx context.Context, nurseID int, date time.Time, in UpsertInput) (*Wish, error) {
	row := r.PG.QueryRow(ctx, `
		INSERT INTO wishes (nurse_id, date, type, reason) VALUES ($1, $2, $3, $4)
		ON CONFLICT (nurse_id, date) DO UPDATE SET type = EXCLUDED.type, reason = EXCLUDED.reason
		RETURNING id, nurse_id, date, type, reason, created_at`,
		nurseID, date, in.Type, in.Reason)
	var w Wish
	if err := row.Scan(&w.ID, &w.NurseID, &w.Date, &w.Type, &w.Reason, &w.CreatedAt); err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *Repo) Delete(ctx context.Context, nurseID int, date time.Time) error {
	_, err := r.PG.Exec(ctx, `DELETE FROM wishes WHERE nurse_id = $1 AND date = $2`, nurseID, date)
	return err
}
