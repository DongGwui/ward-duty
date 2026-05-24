package notifications

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct{ PG *pgxpool.Pool }

func NewRepo(pg *pgxpool.Pool) *Repo { return &Repo{PG: pg} }

const cols = `id, type, title, body, link, meta, read_at, created_at`

func (r *Repo) List(ctx context.Context, nurseID int, unreadOnly bool, limit int) ([]Notification, error) {
	q := `SELECT ` + cols + ` FROM notifications WHERE recipient_nurse_id = $1`
	args := []any{nurseID}
	if unreadOnly {
		q += ` AND read_at IS NULL`
	}
	q += ` ORDER BY created_at DESC`
	if limit > 0 {
		q += ` LIMIT $2`
		args = append(args, limit)
	}
	rows, err := r.PG.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Notification
	for rows.Next() {
		var n Notification
		var meta []byte
		if err := rows.Scan(&n.ID, &n.Type, &n.Title, &n.Body, &n.Link, &meta, &n.ReadAt, &n.CreatedAt); err != nil {
			return nil, err
		}
		if len(meta) > 0 {
			n.Meta = json.RawMessage(meta)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *Repo) UnreadCount(ctx context.Context, nurseID int) (int, error) {
	var c int
	err := r.PG.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE recipient_nurse_id = $1 AND read_at IS NULL`, nurseID).Scan(&c)
	return c, err
}

func (r *Repo) MarkRead(ctx context.Context, nurseID, id int) error {
	res, err := r.PG.Exec(ctx,
		`UPDATE notifications SET read_at = NOW() WHERE id = $1 AND recipient_nurse_id = $2 AND read_at IS NULL`,
		id, nurseID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) MarkAllRead(ctx context.Context, nurseID int) (int, error) {
	res, err := r.PG.Exec(ctx,
		`UPDATE notifications SET read_at = NOW() WHERE recipient_nurse_id = $1 AND read_at IS NULL`, nurseID)
	if err != nil {
		return 0, err
	}
	return int(res.RowsAffected()), nil
}

func (r *Repo) Delete(ctx context.Context, nurseID, id int) error {
	res, err := r.PG.Exec(ctx,
		`DELETE FROM notifications WHERE id = $1 AND recipient_nurse_id = $2`, id, nurseID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Insert — service 또는 트리거에서 호출.
func (r *Repo) Insert(ctx context.Context, in Create) error {
	var metaBytes []byte
	if len(in.Meta) > 0 {
		metaBytes, _ = json.Marshal(in.Meta)
	}
	var body, link *string
	if in.Body != "" {
		body = &in.Body
	}
	if in.Link != "" {
		link = &in.Link
	}
	_, err := r.PG.Exec(ctx, `
		INSERT INTO notifications (recipient_nurse_id, type, title, body, link, meta)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb)`,
		in.RecipientNurseID, in.Type, in.Title, body, link, metaBytes)
	return err
}

// InsertMany — 같은 알림을 여러 수신자에게 (예: 모든 매니저).
func (r *Repo) InsertMany(ctx context.Context, recipients []int, template Create) error {
	if len(recipients) == 0 {
		return nil
	}
	tx, err := r.PG.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var metaBytes []byte
	if len(template.Meta) > 0 {
		metaBytes, _ = json.Marshal(template.Meta)
	}
	batch := &pgx.Batch{}
	for _, nid := range recipients {
		batch.Queue(`INSERT INTO notifications (recipient_nurse_id, type, title, body, link, meta) VALUES ($1, $2, $3, $4, $5, $6::jsonb)`,
			nid, template.Type, template.Title, nullable(template.Body), nullable(template.Link), metaBytes)
	}
	br := tx.SendBatch(ctx, batch)
	for range recipients {
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			return err
		}
	}
	if err := br.Close(); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// HeadNurseIDs — pending/swap-approval 등에서 매니저 일괄 통지용.
func (r *Repo) HeadNurseIDs(ctx context.Context) ([]int, error) {
	rows, err := r.PG.Query(ctx, `SELECT id FROM nurses WHERE role = 'head_nurse' AND active = TRUE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

var ErrNotFound = errors.New("notification not found")

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
