"use client";
import { useState, useMemo } from "react";
import { useParams, useRouter } from "next/navigation";
import { useSchedule, useCells, useNurses, useGenerate, usePatchCell, useConfirm, useReset } from "@/features/duty/hooks/useDuty";
import { SchedulerGrid } from "@/features/duty/components/SchedulerGrid";
import { ViolationSidebar } from "@/features/duty/components/ViolationSidebar";
import { StatusBar } from "@/features/duty/components/StatusBar";
import { PdfExportButton } from "@/features/duty/components/PdfExportButton";
import { Button } from "@/components/ui/Button";
import { Spinner } from "@/components/ui/Spinner";
import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import type { Subject, Violation } from "@/lib/types";

export default function DutyPage() {
  const params = useParams<{ yyyymm: string }>();
  const router = useRouter();
  const ym = params.yyyymm;

  const { data: me } = useQuery<Subject>({ queryKey: ["me"], queryFn: () => apiFetch<Subject>("/auth/me") });
  const isHead = me?.rl === "head_nurse";

  const schedule = useSchedule(ym);
  const nurses = useNurses();
  const cells = useCells(schedule.data?.id, schedule.data?.status === "generated" || schedule.data?.status === "confirmed");

  const generate = useGenerate(ym);
  const patchCell = usePatchCell();
  const confirm = useConfirm();
  const reset = useReset();

  const [violationsState, setViolations] = useState<{ list: Violation[]; hard: number; soft: number }>({
    list: [],
    hard: 0,
    soft: 0,
  });

  const status = schedule.data?.status;
  const canEdit = isHead && status === "generated";

  const dateRange = useMemo(() => {
    const next = new Date(ym + "-01");
    const prev = new Date(ym + "-01");
    next.setMonth(next.getMonth() + 1);
    prev.setMonth(prev.getMonth() - 1);
    const fmt = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
    return { prev: fmt(prev), next: fmt(next) };
  }, [ym]);

  function ScheduleControls() {
    if (!isHead) return null;
    const scheduleId = schedule.data?.id;
    if (status === undefined || schedule.error) {
      // schedule 없음 또는 에러 → Generate 가능
      return (
        <Button onClick={() => generate.mutate()} disabled={generate.isPending}>
          {generate.isPending ? "요청 중…" : "자동 생성"}
        </Button>
      );
    }
    if (status === "draft") {
      return (
        <Button onClick={() => generate.mutate()} disabled={generate.isPending}>
          자동 생성
        </Button>
      );
    }
    if (status === "failed") {
      return (
        <div className="flex gap-2">
          <Button variant="secondary" onClick={() => scheduleId && reset.mutate(scheduleId)}>
            초기화
          </Button>
          <Button onClick={() => generate.mutate()}>재시도</Button>
        </div>
      );
    }
    if (status === "generated") {
      return (
        <Button
          variant="primary"
          onClick={() => scheduleId && confirm.mutate(scheduleId)}
          disabled={confirm.isPending || violationsState.hard > 0}
          title={violationsState.hard > 0 ? "hard 위반이 있어 확정 불가" : ""}
        >
          {confirm.isPending ? "확정 중…" : "확정"}
        </Button>
      );
    }
    return null;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="sm" onClick={() => router.push(`/duty/${dateRange.prev}`)}>
            ←
          </Button>
          <h1 className="text-xl font-bold">{ym}</h1>
          <Button variant="ghost" size="sm" onClick={() => router.push(`/duty/${dateRange.next}`)}>
            →
          </Button>
        </div>
        <div className="flex items-center gap-2">
          <PdfExportButton
            targetId="duty-grid-capture"
            filename={`duty-${ym}`}
            format="pdf"
            disabled={!cells.data}
          />
          <PdfExportButton
            targetId="duty-grid-capture"
            filename={`duty-${ym}`}
            format="png"
            disabled={!cells.data}
          />
          <ScheduleControls />
        </div>
      </div>

      <StatusBar schedule={schedule.data} />

      {confirm.error && (
        <div className="bg-red-50 border border-red-200 rounded-lg px-4 py-3 text-sm text-red-900">
          확정 실패: hard 위반이 남아 있습니다. 사이드바의 위반을 모두 해소한 뒤 다시 시도하세요.
        </div>
      )}

      <div className="grid lg:grid-cols-[1fr_320px] gap-4">
        <div id="duty-grid-capture">
          {schedule.isLoading || nurses.isLoading ? (
            <div className="flex justify-center py-12">
              <Spinner size={6} />
            </div>
          ) : !schedule.data || schedule.data.status === "draft" || schedule.data.status === "failed" ? (
            <div className="bg-white rounded-xl border p-10 text-center text-gray-500">
              {isHead ? '"자동 생성"을 클릭해 듀티를 만드세요.' : "아직 생성되지 않았습니다."}
            </div>
          ) : (
            cells.data && (
              <SchedulerGrid
                yearMonth={ym}
                nurses={nurses.data ?? []}
                cells={cells.data}
                violations={violationsState.list}
                canEdit={canEdit}
                currentNurseId={me?.nid}
                onPatch={(cellId, shift) => {
                  if (!schedule.data) return;
                  patchCell.mutate(
                    { scheduleId: schedule.data.id, cellId, shift },
                    {
                      onSuccess: (res) => {
                        const m = res.meta as
                          | { violations?: Violation[]; hard_count?: number; soft_count?: number }
                          | undefined;
                        setViolations({
                          list: m?.violations ?? [],
                          hard: m?.hard_count ?? 0,
                          soft: m?.soft_count ?? 0,
                        });
                      },
                    },
                  );
                }}
              />
            )
          )}
        </div>

        <aside className="space-y-3">
          <ViolationSidebar
            violations={violationsState.list}
            hardCount={violationsState.hard}
            softCount={violationsState.soft}
          />
        </aside>
      </div>
    </div>
  );
}
