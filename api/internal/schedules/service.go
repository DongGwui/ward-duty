// 솔버 입력 빌더 + 비동기 dispatch + 수동 수정 검증.
//
// Design Ref: §2.2 Data Flow A·B, §4.3, §9.5 룰 매핑
package schedules

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ward-duty-api/internal/notifications"
	"ward-duty-api/internal/nurses"
	"ward-duty-api/internal/solver"
)

// Service — schedule 생성·확정·수동수정 오케스트레이션.
type Service struct {
	PG     *pgxpool.Pool
	Repo   *Repo
	Solver *solver.Client
	Notif  *notifications.Repo // Phase B — schedule_confirmed broadcast
}

func NewService(pg *pgxpool.Pool, sc *solver.Client) *Service {
	return &Service{PG: pg, Repo: NewRepo(pg), Solver: sc}
}

// WithNotif — main.go에서 주입. nil이면 broadcast 안 함.
func (s *Service) WithNotif(n *notifications.Repo) *Service { s.Notif = n; return s }

// Generate — POST /api/schedules dispatch.
//
//   1) InsertGenerating: 새/failed/draft → 'generating'으로 전환
//   2) confirmed 또는 신선 generating → 적절한 sentinel error
//   3) goroutine: solver 호출 → cells UPSERT 또는 failed로 마킹
func (s *Service) Generate(ctx context.Context, ym string) (*Schedule, error) {
	sch, err := s.Repo.InsertGenerating(ctx, ym)
	if err != nil {
		return nil, err
	}
	switch sch.Status {
	case "confirmed":
		return sch, ErrAlreadyConfirmed
	case "generating":
		// 새로 진입된 'generating' (방금 INSERT 또는 transitionToGenerating)이면 dispatch
		// 신선한 기존 'generating'이면 dispatch 안 함 (대신 caller가 ErrInProgress 처리)
		// MVP 단순화: 항상 dispatch 안 하고 status 그대로 반환 → 운영자가 fail로 강제 후 재시도
		// (Web에서는 status 폴링으로 처리)
	}
	// 비동기 분기 — request context 종료 후에도 실행되도록 background context.
	go s.runSolver(context.Background(), sch.ID, ym)
	return sch, nil
}

// ForceReset — 운영자가 stale 'generating' 또는 'failed'를 'draft'로 되돌릴 때.
//
// C5 보조: REST 엔드포인트(POST /api/schedules/:id/reset)에서 호출.
func (s *Service) ForceReset(ctx context.Context, scheduleID int) error {
	return s.Repo.ResetToDraft(ctx, scheduleID)
}

func (s *Service) runSolver(ctx context.Context, scheduleID int, ym string) {
	log := slog.With("schedule_id", scheduleID, "ym", ym)
	log.Info("solver dispatch start")

	input, err := s.buildSolverInput(ctx, ym)
	if err != nil {
		log.Error("build solver input", "err", err)
		_ = s.Repo.UpdateStatusFailed(ctx, scheduleID, &GenerationLog{
			SolverStatus: "input_error",
			Suggestion:   err.Error(),
		})
		return
	}

	out, err := s.Solver.Generate(ctx, *input)
	// H3 fix (Checkpoint 1): out이 nil일 수 있음 — Generate가 post() 실패 시 nil 반환
	if err != nil && !errors.Is(err, solver.ErrInfeasible) {
		log.Error("solver call", "err", err)
		elapsed := 0
		if out != nil {
			elapsed = out.ElapsedMs
		}
		_ = s.Repo.UpdateStatusFailed(ctx, scheduleID, &GenerationLog{
			SolverStatus: "error",
			Suggestion:   err.Error(),
			ElapsedMs:    elapsed,
		})
		return
	}
	if out == nil {
		log.Error("solver returned nil output without error")
		_ = s.Repo.UpdateStatusFailed(ctx, scheduleID, &GenerationLog{
			SolverStatus: "error",
			Suggestion:   "solver returned no output",
		})
		return
	}

	if out.Status != "ok" {
		log.Warn("solver result not ok", "status", out.Status)
		suggestion := ""
		if out.Suggestion != nil {
			suggestion = *out.Suggestion
		}
		_ = s.Repo.UpdateStatusFailed(ctx, scheduleID, &GenerationLog{
			SolverStatus:    out.SolverStatus,
			ViolatedRuleIDs: out.ViolatedRuleIDs,
			Suggestion:      suggestion,
			ElapsedMs:       out.ElapsedMs,
			AppliedRules:    out.AppliedRules,
		})
		return
	}

	// 성공: cells UPSERT
	cells := make([]ScheduleCell, len(out.Cells))
	for i, c := range out.Cells {
		cells[i] = ScheduleCell{NurseID: c.NurseID, Date: c.Date, Shift: c.Shift}
	}
	if err := s.Repo.UpsertCellsAuto(ctx, scheduleID, cells); err != nil {
		log.Error("upsert cells", "err", err)
		_ = s.Repo.UpdateStatusFailed(ctx, scheduleID, &GenerationLog{
			SolverStatus: "persist_error",
			Suggestion:   err.Error(),
		})
		return
	}

	_ = s.Repo.UpdateStatusGenerated(ctx, scheduleID, &GenerationLog{
		SolverStatus: out.SolverStatus,
		ElapsedMs:    out.ElapsedMs,
		AppliedRules: out.AppliedRules,
	})
	log.Info("solver dispatch done", "cells", len(out.Cells), "elapsed_ms", out.ElapsedMs)
}

// buildSolverInput — Design §2.2 자동 생성 흐름의 1~7단계.
func (s *Service) buildSolverInput(ctx context.Context, ym string) (*solver.GenerateInput, error) {
	// 1) ward_settings
	st, err := loadSettings(ctx, s.PG)
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}

	// 2) experience_levels
	lvs, err := loadLevels(ctx, s.PG)
	if err != nil {
		return nil, fmt.Errorf("load levels: %w", err)
	}

	// 3) nurses (active + 등급 자동 분류)
	ns, err := loadActiveNurses(ctx, s.PG)
	if err != nil {
		return nil, fmt.Errorf("load nurses: %w", err)
	}

	// 4) night_keepers for ym
	nkIDs, err := loadNightKeeperIDs(ctx, s.PG, ym)
	if err != nil {
		return nil, fmt.Errorf("load night_keepers: %w", err)
	}

	today := time.Now()
	solverNurses := make([]solver.NurseIn, 0, len(ns))
	for _, n := range ns {
		level := nurses.ClassifyLevel(&n.nurse, lvs.toLevels(), today)
		var fp *string
		if n.fixedShift != nil && *n.fixedShift != "" {
			fp = n.fixedShift
		}
		solverNurses = append(solverNurses, solver.NurseIn{
			ID:            n.id,
			Level:         level,
			FixedPattern:  fp,
			IsNightKeeper: nkIDs[n.id],
		})
	}

	// 5) wishes (ym)
	wsh, err := loadWishesForMonth(ctx, s.PG, ym)
	if err != nil {
		return nil, fmt.Errorf("load wishes: %w", err)
	}

	// 6) previous month last lookback days
	prev, err := s.Repo.PreviousMonthLastWeekCells(ctx, ym, st.PreviousMonthLookbackDays)
	if err != nil {
		return nil, fmt.Errorf("load prev cells: %w", err)
	}
	prevIn := make([]solver.PrevCellIn, 0, len(prev))
	for _, c := range prev {
		prevIn = append(prevIn, solver.PrevCellIn{NurseID: c.NurseID, Date: c.Date, Shift: c.Shift})
	}

	return &solver.GenerateInput{
		YearMonth:                  ym,
		Nurses:                     solverNurses,
		Wishes:                     wsh,
		PreviousMonthLastWeekCells: prevIn,
		ExperienceLevels:           lvs.toSolver(),
		WardSettings:               st.toSolver(),
		MaxTimeSeconds:             55,
	}, nil
}

// ValidateAfterChange — PATCH 셀 후 검증 (Design §2.2 B).
//
// 전체 cells + 컨텍스트를 솔버 /validate로 보냄.
func (s *Service) ValidateAfterChange(ctx context.Context, scheduleID int, ym string) (*solver.ValidateOutput, error) {
	st, err := loadSettings(ctx, s.PG)
	if err != nil {
		return nil, err
	}
	lvs, err := loadLevels(ctx, s.PG)
	if err != nil {
		return nil, err
	}
	ns, err := loadActiveNurses(ctx, s.PG)
	if err != nil {
		return nil, err
	}
	nkIDs, err := loadNightKeeperIDs(ctx, s.PG, ym)
	if err != nil {
		return nil, err
	}
	today := time.Now()
	solverNurses := make([]solver.NurseIn, 0, len(ns))
	for _, n := range ns {
		level := nurses.ClassifyLevel(&n.nurse, lvs.toLevels(), today)
		var fp *string
		if n.fixedShift != nil && *n.fixedShift != "" {
			fp = n.fixedShift
		}
		solverNurses = append(solverNurses, solver.NurseIn{
			ID:            n.id,
			Level:         level,
			FixedPattern:  fp,
			IsNightKeeper: nkIDs[n.id],
		})
	}
	wsh, err := loadWishesForMonth(ctx, s.PG, ym)
	if err != nil {
		return nil, err
	}
	prev, err := s.Repo.PreviousMonthLastWeekCells(ctx, ym, st.PreviousMonthLookbackDays)
	if err != nil {
		return nil, err
	}
	prevIn := make([]solver.PrevCellIn, 0, len(prev))
	for _, c := range prev {
		prevIn = append(prevIn, solver.PrevCellIn{NurseID: c.NurseID, Date: c.Date, Shift: c.Shift})
	}
	cells, err := s.Repo.ListCells(ctx, scheduleID)
	if err != nil {
		return nil, err
	}
	allCells := make([]solver.CellOut, len(cells))
	for i, c := range cells {
		allCells[i] = solver.CellOut{NurseID: c.NurseID, Date: c.Date, Shift: c.Shift}
	}
	in := solver.ValidateInput{
		YearMonth:                  ym,
		Nurses:                     solverNurses,
		Wishes:                     wsh,
		PreviousMonthLastWeekCells: prevIn,
		ExperienceLevels:           lvs.toSolver(),
		WardSettings:               st.toSolver(),
		AllCells:                   allCells,
	}
	return s.Solver.Validate(ctx, in)
}

// Confirm — hard 위반 0건일 때만 'confirmed'로.
func (s *Service) Confirm(ctx context.Context, scheduleID int) error {
	sch, err := s.Repo.Get(ctx, scheduleID)
	if err != nil {
		return err
	}
	if sch.Status != "generated" {
		return ErrCannotConfirm
	}
	v, err := s.ValidateAfterChange(ctx, scheduleID, sch.YearMonth)
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	if v.HardCount > 0 {
		return &HardViolationsError{Violations: v.Violations, HardCount: v.HardCount}
	}
	if err := s.Repo.UpdateStatusConfirmed(ctx, scheduleID); err != nil {
		return err
	}
	// 확정 broadcast — 모든 active nurse에게.
	if s.Notif != nil {
		ids, err := s.Notif.ActiveNurseIDs(ctx)
		if err != nil {
			slog.Warn("schedule_confirmed: list active", "err", err)
			return nil
		}
		if err := s.Notif.InsertMany(ctx, ids, notifications.Create{
			Type:  notifications.TypeScheduleConfirmed,
			Title: sch.YearMonth + " 듀티가 확정되었습니다",
			Body:  "이번 달 근무표를 확인해 주세요.",
			Link:  "/duty/" + sch.YearMonth,
			Meta:  map[string]any{"schedule_id": scheduleID, "year_month": sch.YearMonth},
		}); err != nil {
			slog.Warn("schedule_confirmed: insert many", "err", err)
		}
	}
	return nil
}

var (
	ErrAlreadyConfirmed = errors.New("schedule already confirmed")
	ErrCannotConfirm    = errors.New("only 'generated' can be confirmed")
)

type HardViolationsError struct {
	Violations []solver.Violation
	HardCount  int
}

func (e *HardViolationsError) Error() string {
	return fmt.Sprintf("hard violations: %d", e.HardCount)
}
