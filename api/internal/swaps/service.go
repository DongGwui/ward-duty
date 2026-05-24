// 상태머신 + X-03 검증.
//
// 상태 전이:
//   pending → b_accepted   (target accept)
//   pending → rejected_by_b (target reject)
//   pending → cancelled    (requester cancel)
//   b_accepted → approved  (head approve, X-03 검증 통과)
//   b_accepted → rejected_by_head (head reject)
package swaps

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"ward-duty-api/internal/auth"
	"ward-duty-api/internal/notifications"
	"ward-duty-api/internal/schedules"
	"ward-duty-api/internal/solver"
)

type Service struct {
	PG    *pgxpool.Pool
	Repo  *Repo
	Sched *schedules.Service
	SRepo *schedules.Repo
	Notif *notifications.Repo
}

func NewService(pg *pgxpool.Pool, sc *solver.Client) *Service {
	return &Service{
		PG:    pg,
		Repo:  NewRepo(pg),
		Sched: schedules.NewService(pg, sc),
		SRepo: schedules.NewRepo(pg),
		Notif: notifications.NewRepo(pg),
	}
}

func (s *Service) notify(ctx context.Context, in notifications.Create) {
	_ = s.Notif.Insert(ctx, in)
}
func (s *Service) notifyAll(ctx context.Context, recipients []int, tpl notifications.Create) {
	_ = s.Notif.InsertMany(ctx, recipients, tpl)
}

// Create — requester가 본인 셀 ↔ target 셀 교환 요청.
func (s *Service) Create(ctx context.Context, sub *auth.Subject, in CreateInput) (*SwapRequest, error) {
	if sub.NurseID == in.TargetNurseID {
		return nil, &TransitionError{Code: "VALIDATION_ERROR", Msg: "requester == target"}
	}
	// 셀 존재 확인
	if _, err := s.SRepo.FindCellByKey(ctx, in.ScheduleID, sub.NurseID, in.RequesterDate.Time()); err != nil {
		return nil, &TransitionError{Code: "VALIDATION_ERROR", Msg: "requester cell not found"}
	}
	if _, err := s.SRepo.FindCellByKey(ctx, in.ScheduleID, in.TargetNurseID, in.TargetDate.Time()); err != nil {
		return nil, &TransitionError{Code: "VALIDATION_ERROR", Msg: "target cell not found"}
	}
	sw, err := s.Repo.Create(ctx, in, sub.NurseID)
	if err != nil {
		return nil, err
	}
	s.notify(ctx, notifications.Create{
		RecipientNurseID: sw.TargetNurseID,
		Type:             notifications.TypeSwapRequestReceived,
		Title:            "근무 교환 요청을 받았습니다",
		Body:             fmt.Sprintf("%s ↔ %s", sw.RequesterDate.Time().Format("2006-01-02"), sw.TargetDate.Time().Format("2006-01-02")),
		Link:             "/swaps",
		Meta:             map[string]any{"swap_id": sw.ID},
	})
	return sw, nil
}

// Accept — target이 수락.
func (s *Service) Accept(ctx context.Context, sub *auth.Subject, id int) (*SwapRequest, error) {
	sw, err := s.Repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if sw.Status != StatusPending {
		return nil, transitionErr(sw.Status, "accept")
	}
	if sub.NurseID != sw.TargetNurseID {
		return nil, &TransitionError{Code: "FORBIDDEN", Msg: "only target can accept"}
	}
	if err := s.Repo.UpdateStatus(ctx, id, StatusBAccepted, nil); err != nil {
		return nil, err
	}
	out, _ := s.Repo.Get(ctx, id)
	if out != nil {
		s.notify(ctx, notifications.Create{
			RecipientNurseID: out.RequesterNurseID,
			Type:             notifications.TypeSwapBAccepted,
			Title:            "상대가 교환 요청을 수락했습니다",
			Body:             "매니저 승인을 기다리는 중입니다.",
			Link:             "/swaps",
			Meta:             map[string]any{"swap_id": out.ID},
		})
		if heads, err := s.Notif.HeadNurseIDs(ctx); err == nil {
			s.notifyAll(ctx, heads, notifications.Create{
				Type:  notifications.TypeSwapBAccepted,
				Title: "근무 교환 승인 대기",
				Body:  "양 당사자가 수락한 교환이 매니저 승인을 기다립니다.",
				Link:  "/swaps",
				Meta:  map[string]any{"swap_id": out.ID},
			})
		}
	}
	return out, nil
}

// RejectByB — target이 거부.
func (s *Service) RejectByB(ctx context.Context, sub *auth.Subject, id int, reason *string) (*SwapRequest, error) {
	sw, err := s.Repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if sw.Status != StatusPending {
		return nil, transitionErr(sw.Status, "reject")
	}
	if sub.NurseID != sw.TargetNurseID {
		return nil, &TransitionError{Code: "FORBIDDEN", Msg: "only target can reject"}
	}
	if err := s.Repo.UpdateStatus(ctx, id, StatusRejectedByB, reason); err != nil {
		return nil, err
	}
	out, _ := s.Repo.Get(ctx, id)
	if out != nil {
		s.notify(ctx, notifications.Create{
			RecipientNurseID: out.RequesterNurseID,
			Type:             notifications.TypeSwapRejected,
			Title:            "교환 요청이 거부되었습니다 (상대)",
			Link:             "/swaps",
			Meta:             map[string]any{"swap_id": out.ID},
		})
	}
	return out, nil
}

// Approve — head_nurse 승인. TX 안에서 X-03 검증.
func (s *Service) Approve(ctx context.Context, sub *auth.Subject, id int) (*SwapRequest, *solver.ValidateOutput, error) {
	if sub.Role != "head_nurse" {
		return nil, nil, &TransitionError{Code: "FORBIDDEN", Msg: "only head_nurse"}
	}
	sw, err := s.Repo.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if sw.Status != StatusBAccepted {
		return nil, nil, transitionErr(sw.Status, "approve")
	}

	// 1) cells 교환 (TX)
	cellA, err := s.SRepo.FindCellByKey(ctx, sw.ScheduleID, sw.RequesterNurseID, sw.RequesterDate.Time())
	if err != nil {
		return nil, nil, err
	}
	cellB, err := s.SRepo.FindCellByKey(ctx, sw.ScheduleID, sw.TargetNurseID, sw.TargetDate.Time())
	if err != nil {
		return nil, nil, err
	}
	if err := s.SRepo.SwapCellShifts(ctx, cellA.ID, cellB.ID, sub.NurseID); err != nil {
		return nil, nil, err
	}

	// 2) X-03: 솔버 /validate
	sch, err := s.SRepo.Get(ctx, sw.ScheduleID)
	if err != nil {
		return nil, nil, err
	}
	vOut, err := s.Sched.ValidateAfterChange(ctx, sw.ScheduleID, sch.YearMonth)
	if err != nil {
		// 검증 자체 실패: cells는 이미 바뀜 — 알림 후 head_nurse 판단.
		return sw, nil, fmt.Errorf("validate: %w", err)
	}
	if vOut.HardCount > 0 {
		// 롤백: cells 원상 복구
		_ = s.SRepo.SwapCellShifts(ctx, cellA.ID, cellB.ID, sub.NurseID)
		msg := fmt.Sprintf("swap 결과 hard 위반 %d건 — 거부됨", vOut.HardCount)
		_ = s.Repo.UpdateStatus(ctx, id, StatusRejectedByHead, &msg)
		out, _ := s.Repo.Get(ctx, id)
		return out, vOut, &TransitionError{Code: "RULE_VIOLATION", Msg: msg}
	}
	if err := s.Repo.UpdateStatus(ctx, id, StatusApproved, nil); err != nil {
		return nil, vOut, err
	}
	out, _ := s.Repo.Get(ctx, id)
	if out != nil {
		s.notifyAll(ctx, []int{out.RequesterNurseID, out.TargetNurseID}, notifications.Create{
			Type:  notifications.TypeSwapApproved,
			Title: "근무 교환이 승인되었습니다",
			Link:  "/swaps",
			Meta:  map[string]any{"swap_id": out.ID},
		})
	}
	return out, vOut, nil
}

// Cancel — requester가 취소.
func (s *Service) Cancel(ctx context.Context, sub *auth.Subject, id int) (*SwapRequest, error) {
	sw, err := s.Repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if sw.Status != StatusPending && sw.Status != StatusBAccepted {
		return nil, transitionErr(sw.Status, "cancel")
	}
	if sub.NurseID != sw.RequesterNurseID {
		return nil, &TransitionError{Code: "FORBIDDEN", Msg: "only requester can cancel"}
	}
	if err := s.Repo.UpdateStatus(ctx, id, StatusCancelled, nil); err != nil {
		return nil, err
	}
	return s.Repo.Get(ctx, id)
}

type TransitionError struct {
	Code string
	Msg  string
}

func (e *TransitionError) Error() string { return e.Code + ": " + e.Msg }

func transitionErr(currentStatus, action string) error {
	return &TransitionError{
		Code: "CONFLICT",
		Msg:  fmt.Sprintf("cannot %s from status=%s", action, currentStatus),
	}
}

var _ = errors.New
