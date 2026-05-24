// Duty API 클라이언트.

import { apiFetch, apiFetchFull } from "@/lib/api";
import type { Schedule, ScheduleCell, Nurse, PatchCellMeta, ShiftCode } from "@/lib/types";

export const dutyApi = {
  // 자동 생성 dispatch (202)
  create: (yearMonth: string) =>
    apiFetch<Schedule>("/schedules", { method: "POST", body: { year_month: yearMonth } }),

  // 월별 schedule 조회 (status 폴링)
  getByYM: (yearMonth: string) => apiFetch<Schedule>("/schedules", { search: { ym: yearMonth } }),

  // 셀 목록
  getCells: (scheduleId: number) => apiFetch<ScheduleCell[]>(`/schedules/${scheduleId}/cells`),

  // 셀 수정 + 검증 결과 (meta.violations)
  patchCell: (scheduleId: number, cellId: number, shift: ShiftCode) =>
    apiFetchFull<ScheduleCell>(`/schedules/${scheduleId}/cells/${cellId}`, {
      method: "PATCH",
      body: { shift },
    }),

  // 확정 (hard 위반 0일 때만 성공)
  confirm: (scheduleId: number) => apiFetch<Schedule>(`/schedules/${scheduleId}/confirm`, { method: "POST" }),

  // 강제 reset (stale generating → draft)
  reset: (scheduleId: number) => apiFetch<Schedule>(`/schedules/${scheduleId}/reset`, { method: "POST" }),

  // 간호사 목록 (그리드 행)
  listNurses: () => apiFetch<Nurse[]>("/nurses"),
};

export type { PatchCellMeta };
