package swaps

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct{ PG *pgxpool.Pool }

func NewRepo(pg *pgxpool.Pool) *Repo { return &Repo{PG: pg} }

const cols = `id, schedule_id, requester_nurse_id, target_nurse_id,
		requester_date, target_date, status, reason, rejected_reason, created_at, updated_at`

func scanSwap(row pgx.Row, s *SwapRequest) error {
	return row.Scan(&s.ID, &s.ScheduleID, &s.RequesterNurseID, &s.TargetNurseID,
		&s.RequesterDate, &s.TargetDate, &s.Status, &s.Reason, &s.RejectedReason,
		&s.CreatedAt, &s.UpdatedAt)
}

func (r *Repo) List(ctx context.Context, opts ListOpts) ([]SwapRequest, error) {
	parts := []string{"1=1"}
	args := []any{}
	if opts.Status != "" {
		args = append(args, opts.Status)
		parts = append(parts, "status = $"+itoa(len(args)))
	}
	if opts.RequesterID > 0 {
		args = append(args, opts.RequesterID)
		parts = append(parts, "requester_nurse_id = $"+itoa(len(args)))
	}
	if opts.TargetID > 0 {
		args = append(args, opts.TargetID)
		parts = append(parts, "target_nurse_id = $"+itoa(len(args)))
	}
	if opts.ScheduleID > 0 {
		args = append(args, opts.ScheduleID)
		parts = append(parts, "schedule_id = $"+itoa(len(args)))
	}
	q := "SELECT " + cols + " FROM swap_requests WHERE " + strings.Join(parts, " AND ") + " ORDER BY created_at DESC"
	rows, err := r.PG.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SwapRequest
	for rows.Next() {
		var s SwapRequest
		if err := scanSwap(rows, &s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

type ListOpts struct {
	Status      string
	RequesterID int
	TargetID    int
	ScheduleID  int
}

func (r *Repo) Get(ctx context.Context, id int) (*SwapRequest, error) {
	row := r.PG.QueryRow(ctx, "SELECT "+cols+" FROM swap_requests WHERE id = $1", id)
	var s SwapRequest
	if err := scanSwap(row, &s); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *Repo) Create(ctx context.Context, in CreateInput, requesterID int) (*SwapRequest, error) {
	row := r.PG.QueryRow(ctx, `
		INSERT INTO swap_requests (schedule_id, requester_nurse_id, target_nurse_id, requester_date, target_date, status, reason)
		VALUES ($1, $2, $3, $4, $5, 'pending', $6)
		RETURNING `+cols,
		in.ScheduleID, requesterID, in.TargetNurseID, in.RequesterDate, in.TargetDate, in.Reason)
	var s SwapRequest
	if err := scanSwap(row, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repo) UpdateStatus(ctx context.Context, id int, status string, rejectedReason *string) error {
	_, err := r.PG.Exec(ctx, `
		UPDATE swap_requests SET status = $1, rejected_reason = $2, updated_at = NOW() WHERE id = $3`,
		status, rejectedReason, id)
	return err
}

var ErrNotFound = errors.New("swap not found")

func itoa(n int) string { return strconv.Itoa(n) }
