"use client";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { dutyApi } from "../api";
import type { Schedule, ScheduleCell, Nurse, ShiftCode, Violation } from "@/lib/types";

// 월별 schedule + 폴링 (status === 'generating'일 때 3초마다)
export function useSchedule(yearMonth: string) {
  return useQuery<Schedule>({
    queryKey: ["schedule", yearMonth],
    queryFn: () => dutyApi.getByYM(yearMonth),
    refetchInterval: (q) => (q.state.data?.status === "generating" ? 3000 : false),
    retry: (_n, err: unknown) => {
      const status = (err as { status?: number })?.status ?? 0;
      return status !== 404 && status !== 401;
    },
  });
}

export function useCells(scheduleId: number | undefined, enabled = true) {
  return useQuery<ScheduleCell[]>({
    queryKey: ["cells", scheduleId],
    queryFn: () => dutyApi.getCells(scheduleId!),
    enabled: enabled && !!scheduleId,
  });
}

export function useNurses() {
  return useQuery<Nurse[]>({ queryKey: ["nurses"], queryFn: () => dutyApi.listNurses() });
}

export function useGenerate(yearMonth: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => dutyApi.create(yearMonth),
    onSuccess: (s) => qc.setQueryData(["schedule", yearMonth], s),
  });
}

export function usePatchCell() {
  const qc = useQueryClient();
  return useMutation<
    { data: ScheduleCell; meta?: { violations?: Violation[]; hard_count?: number; soft_count?: number } },
    Error,
    { scheduleId: number; cellId: number; shift: ShiftCode }
  >({
    mutationFn: ({ scheduleId, cellId, shift }) => dutyApi.patchCell(scheduleId, cellId, shift),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ["cells", vars.scheduleId] });
    },
  });
}

export function useConfirm() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (scheduleId: number) => dutyApi.confirm(scheduleId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["schedule"] });
      qc.invalidateQueries({ queryKey: ["cells"] });
    },
  });
}

export function useReset() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (scheduleId: number) => dutyApi.reset(scheduleId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["schedule"] }),
  });
}
