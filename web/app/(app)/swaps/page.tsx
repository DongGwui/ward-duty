"use client";
import { useState, useMemo } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch, ApiException } from "@/lib/api";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import type { Subject, Nurse, SwapStatus, Schedule, ScheduleCell, ShiftCode } from "@/lib/types";
import { SHIFT_BG, SHIFT_TEXT } from "@/lib/shifts";
import clsx from "clsx";

interface SwapRequest {
  id: number; schedule_id: number;
  requester_nurse_id: number; target_nurse_id: number;
  requester_date: string; target_date: string;
  status: SwapStatus; reason?: string | null; rejected_reason?: string | null;
  created_at: string; updated_at: string;
}

const STATUS_LABEL: Record<SwapStatus, string> = {
  pending: "대기 중",
  b_accepted: "수락됨 · 수간호사 승인 대기",
  approved: "승인 완료",
  rejected_by_b: "상대 거부",
  rejected_by_head: "수간호사 거부",
  cancelled: "취소됨",
};
const STATUS_VARIANT: Record<SwapStatus, "default" | "warning" | "success" | "danger" | "info"> = {
  pending: "warning",
  b_accepted: "info",
  approved: "success",
  rejected_by_b: "danger",
  rejected_by_head: "danger",
  cancelled: "default",
};

function thisMonth() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

// ============================================================
// Page
// ============================================================

export default function SwapsPage() {
  const { data: me } = useQuery<Subject>({ queryKey: ["me"], queryFn: () => apiFetch<Subject>("/auth/me") });
  const isHead = me?.rl === "head_nurse";
  const nurses = useQuery<Nurse[]>({ queryKey: ["nurses"], queryFn: () => apiFetch<Nurse[]>("/nurses") });
  const nMap = new Map((nurses.data ?? []).map((n) => [n.id, n.name]));

  const inbox = useQuery<SwapRequest[]>({
    queryKey: ["swaps", "target=me"],
    queryFn: () => apiFetch<SwapRequest[]>("/swaps", { search: { target: "me" } }),
    enabled: !!me,
  });
  const sent = useQuery<SwapRequest[]>({
    queryKey: ["swaps", "requester=me"],
    queryFn: () => apiFetch<SwapRequest[]>("/swaps", { search: { requester: "me" } }),
    enabled: !!me,
  });
  const pendingApproval = useQuery<SwapRequest[]>({
    queryKey: ["swaps", "status=b_accepted"],
    queryFn: () => apiFetch<SwapRequest[]>("/swaps", { search: { status: "b_accepted" } }),
    enabled: isHead,
  });

  const qc = useQueryClient();
  const patch = useMutation({
    mutationFn: ({ id, action, reason }: { id: number; action: "accept" | "reject" | "approve"; reason?: string }) =>
      apiFetch<SwapRequest>(`/swaps/${id}`, { method: "PATCH", body: { action, reason } }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["swaps"] }),
  });
  const cancel = useMutation({
    mutationFn: (id: number) => apiFetch<SwapRequest>(`/swaps/${id}/cancel`, { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["swaps"] }),
  });

  const [creating, setCreating] = useState(false);

  if (!me || nurses.isLoading) return <Spinner size={6} />;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-bold">근무 교환</h1>
          <p className="text-sm text-gray-600 mt-1 max-w-xl">
            근무를 다른 간호사와 바꿔야 할 때 사용합니다. <b>요청</b> → <b>상대 수락</b> → <b>수간호사 승인</b> 순서로 진행됩니다.
          </p>
        </div>
        <Button onClick={() => setCreating(true)} className="shrink-0">
          + 새 요청
        </Button>
      </div>

      {patch.error && <ErrorBox err={patch.error as ApiException} />}

      <SwapSection
        title="받은 요청"
        icon="📥"
        accent="warning"
        items={inbox.data}
        emptyText="다른 간호사가 나에게 보낸 교환 요청이 여기 표시됩니다."
        kind="inbox"
        nMap={nMap}
        patch={patch.mutate}
        cancel={cancel.mutate}
      />

      <SwapSection
        title="보낸 요청"
        icon="📤"
        accent="default"
        items={sent.data}
        emptyText="내가 보낸 교환 요청이 여기 표시됩니다. 우상단 “+ 새 요청”으로 시작하세요."
        kind="sent"
        nMap={nMap}
        patch={patch.mutate}
        cancel={cancel.mutate}
      />

      {isHead && (
        <SwapSection
          title="승인 대기"
          subtitle="수간호사"
          icon="✅"
          accent="info"
          items={pendingApproval.data}
          emptyText="양 당사자가 수락한 교환이 여기서 최종 승인을 기다립니다. 승인 시 X-03 규칙 검증이 자동 실행됩니다."
          kind="approval"
          nMap={nMap}
          patch={patch.mutate}
          cancel={cancel.mutate}
        />
      )}

      {creating && (
        <SwapCreateModal
          me={me}
          nurses={(nurses.data ?? []).filter((n) => n.active && n.id !== me.nid)}
          onClose={() => setCreating(false)}
          onCreated={() => {
            qc.invalidateQueries({ queryKey: ["swaps"] });
            setCreating(false);
          }}
        />
      )}
    </div>
  );
}

// ============================================================
// Section (카드 wrapper + 빈 상태 + 카드 리스트)
// ============================================================

interface SectionProps {
  title: string;
  subtitle?: string;
  icon: string;
  accent: "warning" | "info" | "default";
  items?: SwapRequest[];
  emptyText: string;
  kind: "inbox" | "sent" | "approval";
  nMap: Map<number, string>;
  patch: (v: { id: number; action: "accept" | "reject" | "approve"; reason?: string }) => void;
  cancel: (id: number) => void;
}

function SwapSection({ title, subtitle, icon, accent, items, emptyText, kind, nMap, patch, cancel }: SectionProps) {
  const accentBar =
    accent === "warning" ? "bg-yellow-400" : accent === "info" ? "bg-blue-500" : "bg-gray-300";
  const count = items?.length ?? 0;
  return (
    <section className="bg-white border rounded-xl overflow-hidden">
      <header className="flex items-center gap-3 px-5 py-3 border-b bg-gray-50/50">
        <div className={clsx("w-1 h-6 rounded", accentBar)} />
        <div className="flex items-baseline gap-2">
          <span className="text-lg">{icon}</span>
          <h2 className="font-semibold">{title}</h2>
          {subtitle && <span className="text-xs text-gray-500">({subtitle})</span>}
        </div>
        <Badge variant={count > 0 ? (accent === "warning" ? "warning" : accent === "info" ? "info" : "default") : "default"}>
          {count}
        </Badge>
      </header>

      <div className="p-5">
        {count === 0 ? (
          <div className="text-center py-8">
            <div className="text-3xl mb-2 opacity-50">{icon}</div>
            <p className="text-sm text-gray-500 max-w-md mx-auto">{emptyText}</p>
          </div>
        ) : (
          <div className="space-y-3">
            {items!.map((s) => (
              <SwapCard key={s.id} swap={s} kind={kind} nMap={nMap} patch={patch} cancel={cancel} />
            ))}
          </div>
        )}
      </div>
    </section>
  );
}

// ============================================================
// Swap Card
// ============================================================

interface CardProps {
  swap: SwapRequest;
  kind: "inbox" | "sent" | "approval";
  nMap: Map<number, string>;
  patch: (v: { id: number; action: "accept" | "reject" | "approve"; reason?: string }) => void;
  cancel: (id: number) => void;
}

function SwapCard({ swap: s, kind, nMap, patch, cancel }: CardProps) {
  return (
    <article className="border rounded-lg p-4 hover:border-gray-300 transition">
      <div className="flex items-center justify-between mb-3 flex-wrap gap-2">
        <Badge variant={STATUS_VARIANT[s.status]}>{STATUS_LABEL[s.status]}</Badge>
        <div className="text-[11px] text-gray-400">{new Date(s.created_at).toLocaleString()}</div>
      </div>

      <div className="flex items-center justify-center gap-3 text-sm py-2">
        <DateChip name={nMap.get(s.requester_nurse_id) ?? `#${s.requester_nurse_id}`} date={s.requester_date} />
        <span className="text-gray-400 text-xl">↔</span>
        <DateChip name={nMap.get(s.target_nurse_id) ?? `#${s.target_nurse_id}`} date={s.target_date} />
      </div>

      {(s.reason || s.rejected_reason) && (
        <div className="mt-3 space-y-1">
          {s.reason && <p className="text-xs text-gray-600">사유: {s.reason}</p>}
          {s.rejected_reason && (
            <p className="text-xs text-red-700">거부 사유: {s.rejected_reason}</p>
          )}
        </div>
      )}

      <div className="flex gap-2 mt-3 justify-end">
        {kind === "inbox" && s.status === "pending" && (
          <>
            <Button size="sm" variant="secondary" onClick={() => patch({ id: s.id, action: "reject" })}>
              거부
            </Button>
            <Button size="sm" onClick={() => patch({ id: s.id, action: "accept" })}>
              수락
            </Button>
          </>
        )}
        {kind === "sent" && (s.status === "pending" || s.status === "b_accepted") && (
          <Button size="sm" variant="ghost" onClick={() => cancel(s.id)}>
            취소
          </Button>
        )}
        {kind === "approval" && s.status === "b_accepted" && (
          <Button size="sm" onClick={() => patch({ id: s.id, action: "approve" })}>
            승인 (X-03 검증)
          </Button>
        )}
      </div>
    </article>
  );
}

function DateChip({ name, date }: { name: string; date: string }) {
  return (
    <div className="text-center">
      <div className="font-medium">{name}</div>
      <div className="font-mono text-xs text-gray-500 mt-0.5">{date}</div>
    </div>
  );
}

// ============================================================
// Swap Create Modal
// ============================================================

interface SwapCreateProps {
  me: Subject;
  nurses: Nurse[];
  onClose: () => void;
  onCreated: () => void;
}

function SwapCreateModal({ me, nurses, onClose, onCreated }: SwapCreateProps) {
  const [ym, setYm] = useState(thisMonth());
  const [targetNurseId, setTargetNurseId] = useState<number | "">("");
  const [requesterDate, setRequesterDate] = useState("");
  const [targetDate, setTargetDate] = useState("");
  const [reason, setReason] = useState("");

  const schedule = useQuery<Schedule>({
    queryKey: ["schedule", ym],
    queryFn: () => apiFetch<Schedule>("/schedules", { search: { ym } }),
    retry: false,
  });
  const cells = useQuery<ScheduleCell[]>({
    queryKey: ["cells", schedule.data?.id],
    queryFn: () => apiFetch<ScheduleCell[]>(`/schedules/${schedule.data!.id}/cells`),
    enabled: !!schedule.data?.id,
  });

  const myShifts = useMemo(
    () => (cells.data ?? []).filter((c) => c.nurse_id === me.nid),
    [cells.data, me.nid],
  );
  const targetShifts = useMemo(
    () => (targetNurseId ? (cells.data ?? []).filter((c) => c.nurse_id === targetNurseId) : []),
    [cells.data, targetNurseId],
  );

  const create = useMutation({
    mutationFn: () =>
      apiFetch<SwapRequest>("/swaps", {
        method: "POST",
        body: {
          schedule_id: schedule.data!.id,
          target_nurse_id: targetNurseId,
          requester_date: requesterDate,
          target_date: targetDate,
          reason: reason || undefined,
        },
      }),
    onSuccess: onCreated,
  });

  const canSubmit =
    !!schedule.data?.id && !!targetNurseId && !!requesterDate && !!targetDate;

  return (
    <Modal open onClose={onClose} title="새 swap 요청" size="lg">
      <div className="space-y-4 text-sm">
        <label className="block">
          <span className="block text-xs text-gray-600 mb-1">월</span>
          <input
            type="month"
            className="input"
            value={ym}
            onChange={(e) => {
              setYm(e.target.value);
              setRequesterDate("");
              setTargetDate("");
            }}
          />
          {schedule.isLoading && <p className="text-xs text-gray-500 mt-1">조회 중…</p>}
          {schedule.error && (
            <p className="text-xs text-red-600 mt-1">해당 월의 schedule이 아직 없습니다.</p>
          )}
          {schedule.data?.status && schedule.data.status !== "confirmed" && (
            <p className="text-xs text-yellow-700 mt-1">
              status={schedule.data.status} — 보통 confirmed 상태에서 swap합니다. 진행은 가능.
            </p>
          )}
        </label>

        <div className="grid sm:grid-cols-2 gap-4">
          <CellPanel
            title="내 셀"
            cells={myShifts}
            selectedDate={requesterDate}
            onSelect={setRequesterDate}
            emptyText="해당 월에 내 셀이 없습니다."
          />

          <div className="border rounded-lg p-3 bg-gray-50">
            <div className="font-semibold mb-2">상대 셀</div>
            <select
              className="input mb-2"
              value={targetNurseId}
              onChange={(e) => {
                setTargetNurseId(e.target.value ? Number(e.target.value) : "");
                setTargetDate("");
              }}
            >
              <option value="">교환할 사람 선택…</option>
              {nurses.map((n) => (
                <option key={n.id} value={n.id}>
                  {n.name} {n.resolved_level ? `(${n.resolved_level})` : ""}
                </option>
              ))}
            </select>
            <CellGrid
              cells={targetShifts}
              selectedDate={targetDate}
              onSelect={setTargetDate}
              emptyText={targetNurseId ? "해당 월에 상대 셀이 없습니다." : "사람을 먼저 선택하세요."}
            />
            <div className="text-xs mt-2 text-gray-600">
              상대 날짜: <span className="font-mono">{targetDate || "-"}</span>
            </div>
          </div>
        </div>

        <label className="block">
          <span className="block text-xs text-gray-600 mb-1">사유 (선택)</span>
          <input className="input" value={reason} onChange={(e) => setReason(e.target.value)} />
        </label>

        {create.error && <ErrorBox err={create.error as ApiException} />}

        <div className="flex justify-end gap-2 pt-2">
          <Button variant="ghost" onClick={onClose}>
            취소
          </Button>
          <Button onClick={() => create.mutate()} disabled={!canSubmit || create.isPending}>
            {create.isPending ? "요청 중…" : "요청 보내기"}
          </Button>
        </div>
      </div>
    </Modal>
  );
}

function CellPanel({
  title,
  cells,
  selectedDate,
  onSelect,
  emptyText,
}: {
  title: string;
  cells: ScheduleCell[];
  selectedDate: string;
  onSelect: (d: string) => void;
  emptyText: string;
}) {
  return (
    <div className="border rounded-lg p-3 bg-gray-50">
      <div className="font-semibold mb-2">{title}</div>
      <CellGrid cells={cells} selectedDate={selectedDate} onSelect={onSelect} emptyText={emptyText} />
      <div className="text-xs mt-2 text-gray-600">
        내 날짜: <span className="font-mono">{selectedDate || "-"}</span>
      </div>
    </div>
  );
}

function CellGrid({
  cells,
  selectedDate,
  onSelect,
  emptyText,
}: {
  cells: ScheduleCell[];
  selectedDate: string;
  onSelect: (d: string) => void;
  emptyText: string;
}) {
  if (cells.length === 0) {
    return <p className="text-xs text-gray-400 py-4 text-center">{emptyText}</p>;
  }
  return (
    <div className="grid grid-cols-7 gap-1 max-h-48 overflow-y-auto">
      {cells.map((c) => (
        <button
          key={c.id}
          onClick={() => onSelect(c.date)}
          className={clsx(
            "rounded text-[10px] py-1 px-0.5 transition font-medium",
            SHIFT_BG[c.shift as ShiftCode],
            SHIFT_TEXT[c.shift as ShiftCode],
            selectedDate === c.date && "ring-2 ring-blue-500",
          )}
          title={`${c.date} ${c.shift}`}
        >
          {Number(c.date.slice(-2))}
        </button>
      ))}
    </div>
  );
}

// ============================================================
// Error Box
// ============================================================

function ErrorBox({ err }: { err: ApiException }) {
  return (
    <div className="bg-red-50 border border-red-200 rounded-lg px-4 py-3 text-sm">
      <div className="font-semibold text-red-900">
        {err.body.rule_id ? `[${err.body.rule_id}] ${err.body.message}` : err.body.message}
      </div>
      {err.body.details && (err.body.details as { hard_count?: number }).hard_count ? (
        <div className="text-xs text-red-800 mt-1">
          hard 위반 {(err.body.details as { hard_count: number }).hard_count}건 — 자동 롤백됨
        </div>
      ) : null}
    </div>
  );
}
