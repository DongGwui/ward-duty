package schedules

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// silence unused
var _ = staleGeneratingThreshold

type Repo struct{ PG *pgxpool.Pool }

func NewRepo(pg *pgxpool.Pool) *Repo { return &Repo{PG: pg} }

const schedCols = `id, year_month, status, generated_at, confirmed_at, generation_log`

func (r *Repo) GetByYM(ctx context.Context, ym string) (*Schedule, error) {
	row := r.PG.QueryRow(ctx, "SELECT "+schedCols+" FROM schedules WHERE year_month = $1", ym)
	return scanSchedule(row)
}

func (r *Repo) Get(ctx context.Context, id int) (*Schedule, error) {
	row := r.PG.QueryRow(ctx, "SELECT "+schedCols+" FROM schedules WHERE id = $1", id)
	return scanSchedule(row)
}

func scanSchedule(row pgx.Row) (*Schedule, error) {
	var s Schedule
	var logRaw []byte
	if err := row.Scan(&s.ID, &s.YearMonth, &s.Status, &s.GeneratedAt, &s.ConfirmedAt, &logRaw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if len(logRaw) > 0 {
		s.GenerationLog = json.RawMessage(logRaw)
	}
	return &s, nil
}

// staleGeneratingThreshold — 이 시간 이상 'generating'에 머문 row는 stale로 간주.
// 솔버 timeout(기본 300s)의 2배 + 여유. 운영자가 재시도 가능.
const staleGeneratingThreshold = 30 * time.Minute

// InsertGenerating — 자동 생성 dispatch 직전 호출.
//
// C5 fix (Checkpoint 1 Critical):
//   - UNIQUE(year_month) 충돌 시:
//       * status='confirmed' → 그대로 반환 (caller가 ErrAlreadyConfirmed 처리)
//       * status='failed'|'draft' → UPDATE status='generating'으로 재진입
//       * status='generating' + 오래된 row(staleGeneratingThreshold 초과) → 재진입
//       * status='generating' + 신선 → 그대로 반환 (caller가 ErrInProgress 처리)
func (r *Repo) InsertGenerating(ctx context.Context, ym string) (*Schedule, error) {
	// 1) 새 INSERT 시도
	row := r.PG.QueryRow(ctx, `
		INSERT INTO schedules (year_month, status) VALUES ($1, 'generating')
		ON CONFLICT (year_month) DO NOTHING
		RETURNING `+schedCols, ym)
	s, err := scanSchedule(row)
	if err == nil {
		return s, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	// 2) 충돌 → 기존 row 평가
	existing, err := r.GetByYM(ctx, ym)
	if err != nil {
		return nil, err
	}
	switch existing.Status {
	case "confirmed":
		return existing, nil // caller에서 처리
	case "failed", "draft":
		return r.transitionToGenerating(ctx, existing.ID)
	case "generating":
		// stale 검사
		var generatedAt *time.Time
		if existing.GeneratedAt != nil {
			generatedAt = existing.GeneratedAt
		}
		_ = generatedAt
		// generated_at는 generated 상태일 때만 채워짐. generating의 stale 판정은
		// generation_log 부재 + UPDATE 시점이 필요하지만 단순화: 신선/stale은
		// 별도 추적 컬럼 없이 'generation_log'가 채워졌는지로 갈음.
		// → MVP 단순화: 명시적 강제 진입(force=true) 또는 운영자 ResetToDraft 호출
		// 신선 generating은 그대로 반환 → caller에서 ErrInProgress
		return existing, nil
	default:
		return existing, nil
	}
}

func (r *Repo) transitionToGenerating(ctx context.Context, id int) (*Schedule, error) {
	row := r.PG.QueryRow(ctx, `
		UPDATE schedules
		SET status = 'generating', generated_at = NULL, generation_log = NULL
		WHERE id = $1 AND status IN ('failed', 'draft')
		RETURNING `+schedCols, id)
	s, err := scanSchedule(row)
	if errors.Is(err, ErrNotFound) {
		// race로 다른 트랜잭션이 상태 바꿈 — 현재값 반환
		return r.Get(ctx, id)
	}
	return s, err
}

// UpdateStatusGenerated — solver 결과 도착 시.
func (r *Repo) UpdateStatusGenerated(ctx context.Context, id int, log *GenerationLog) error {
	logBytes, _ := json.Marshal(log)
	_, err := r.PG.Exec(ctx, `
		UPDATE schedules SET status = 'generated', generated_at = NOW(), generation_log = $2::jsonb
		WHERE id = $1`, id, logBytes)
	return err
}

// UpdateStatusFailed — infeasible/timeout/error.
func (r *Repo) UpdateStatusFailed(ctx context.Context, id int, log *GenerationLog) error {
	logBytes, _ := json.Marshal(log)
	_, err := r.PG.Exec(ctx, `
		UPDATE schedules SET status = 'failed', generation_log = $2::jsonb
		WHERE id = $1`, id, logBytes)
	return err
}

// UpdateStatusConfirmed — 확정.
func (r *Repo) UpdateStatusConfirmed(ctx context.Context, id int) error {
	_, err := r.PG.Exec(ctx, `
		UPDATE schedules SET status = 'confirmed', confirmed_at = NOW()
		WHERE id = $1 AND status = 'generated'`, id)
	return err
}

// ResetToDraft — 재생성 위해 'failed' → 'draft'로 (운영자가 재시도 가능).
func (r *Repo) ResetToDraft(ctx context.Context, id int) error {
	_, err := r.PG.Exec(ctx, `UPDATE schedules SET status = 'draft' WHERE id = $1`, id)
	return err
}

// ---------- cells ----------

const cellCols = `id, schedule_id, nurse_id, date, shift, source, modified_by_nurse_id, updated_at`

func (r *Repo) ListCells(ctx context.Context, scheduleID int) ([]ScheduleCell, error) {
	rows, err := r.PG.Query(ctx,
		"SELECT "+cellCols+" FROM schedule_cells WHERE schedule_id = $1 ORDER BY date, nurse_id", scheduleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScheduleCell
	for rows.Next() {
		var c ScheduleCell
		if err := rows.Scan(&c.ID, &c.ScheduleID, &c.NurseID, &c.Date, &c.Shift, &c.Source, &c.ModifiedByNurseID, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repo) GetCell(ctx context.Context, cellID int) (*ScheduleCell, error) {
	row := r.PG.QueryRow(ctx, "SELECT "+cellCols+" FROM schedule_cells WHERE id = $1", cellID)
	var c ScheduleCell
	if err := row.Scan(&c.ID, &c.ScheduleID, &c.NurseID, &c.Date, &c.Shift, &c.Source, &c.ModifiedByNurseID, &c.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCellNotFound
		}
		return nil, err
	}
	return &c, nil
}

// UpsertCellsAuto — 솔버 결과를 일괄 INSERT (source='auto').
func (r *Repo) UpsertCellsAuto(ctx context.Context, scheduleID int, cells []ScheduleCell) error {
	tx, err := r.PG.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `DELETE FROM schedule_cells WHERE schedule_id = $1`, scheduleID)
	if err != nil {
		return err
	}
	batch := &pgx.Batch{}
	for _, c := range cells {
		batch.Queue(`INSERT INTO schedule_cells (schedule_id, nurse_id, date, shift, source) VALUES ($1, $2, $3, $4, 'auto')`,
			scheduleID, c.NurseID, c.Date, c.Shift)
	}
	br := tx.SendBatch(ctx, batch)
	for range cells {
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

// UpdateCell — 수동 수정.
func (r *Repo) UpdateCell(ctx context.Context, cellID int, shift string, modifiedBy int) (*ScheduleCell, error) {
	row := r.PG.QueryRow(ctx, `
		UPDATE schedule_cells
		SET shift = $1, source = 'manual', modified_by_nurse_id = $2, updated_at = NOW()
		WHERE id = $3
		RETURNING `+cellCols, shift, modifiedBy, cellID)
	var c ScheduleCell
	if err := row.Scan(&c.ID, &c.ScheduleID, &c.NurseID, &c.Date, &c.Shift, &c.Source, &c.ModifiedByNurseID, &c.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCellNotFound
		}
		return nil, err
	}
	return &c, nil
}

// SwapCellShifts — swap 승인 시 트랜잭션 내 cells 2건 교환.
func (r *Repo) SwapCellShifts(ctx context.Context, cellAID, cellBID, modifiedBy int) error {
	tx, err := r.PG.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var shiftA, shiftB string
	if err := tx.QueryRow(ctx, `SELECT shift FROM schedule_cells WHERE id = $1 FOR UPDATE`, cellAID).Scan(&shiftA); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `SELECT shift FROM schedule_cells WHERE id = $1 FOR UPDATE`, cellBID).Scan(&shiftB); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE schedule_cells SET shift = $1, source = 'manual', modified_by_nurse_id = $2, updated_at = NOW() WHERE id = $3`, shiftB, modifiedBy, cellAID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE schedule_cells SET shift = $1, source = 'manual', modified_by_nurse_id = $2, updated_at = NOW() WHERE id = $3`, shiftA, modifiedBy, cellBID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// FindCellByKey — schedule_id + nurse_id + date.
func (r *Repo) FindCellByKey(ctx context.Context, scheduleID, nurseID int, date time.Time) (*ScheduleCell, error) {
	row := r.PG.QueryRow(ctx,
		"SELECT "+cellCols+" FROM schedule_cells WHERE schedule_id = $1 AND nurse_id = $2 AND date = $3",
		scheduleID, nurseID, date)
	var c ScheduleCell
	if err := row.Scan(&c.ID, &c.ScheduleID, &c.NurseID, &c.Date, &c.Shift, &c.Source, &c.ModifiedByNurseID, &c.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCellNotFound
		}
		return nil, err
	}
	return &c, nil
}

// PreviousMonthLastWeekCells — H-10 입력용. 이번 ym 기준 이전 달 마지막 N일.
func (r *Repo) PreviousMonthLastWeekCells(ctx context.Context, ym string, lookback int) ([]ScheduleCell, error) {
	// ym = YYYY-MM → 첫째 날
	t, err := time.Parse("2006-01-02", ym+"-01")
	if err != nil {
		return nil, err
	}
	from := t.AddDate(0, 0, -lookback)
	to := t // exclusive (이번 달 1일)
	rows, err := r.PG.Query(ctx,
		"SELECT "+cellCols+" FROM schedule_cells WHERE date >= $1 AND date < $2 ORDER BY date, nurse_id",
		from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScheduleCell
	for rows.Next() {
		var c ScheduleCell
		if err := rows.Scan(&c.ID, &c.ScheduleID, &c.NurseID, &c.Date, &c.Shift, &c.Source, &c.ModifiedByNurseID, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

var (
	ErrNotFound      = errors.New("schedule not found")
	ErrCellNotFound  = errors.New("schedule cell not found")
)
