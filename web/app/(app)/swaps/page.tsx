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
  pending: "대기",
  b_accepted: "B 수락 — 수간호사 승인 대기",
  approved: "승인",
  rejected_by_b: "B 거부",
  rejected_by_head: "수간호사 거부",
  cancelled: "취소",
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

  const CardList = ({ title, items, kind }: { title: string; items?: SwapRequest[]; kind: "inbox" | "sent" | "approval" }) => (
    <section className="space-y-2">
      <h2 className="font-semibold">{title} <span className="text-gray-400 text-sm">({items?.length ?? 0})</span></h2>
      {(items ?? []).length === 0 ? (
        <p className="text-sm text-gray-400">없음</p>
      ) : (
        items!.map((s) => (
          <article key={s.id} className="bg-white border rounded-lg p-4">
            <div className="flex items-center justify-between mb-2">
              <Badge variant={STATUS_VARIANT[s.status]}>{STATUS_LABEL[s.status]}</Badge>
              <div className="text-xs text-gray-400">{new Date(s.created_at).toLocaleString()}</div>
            </div>
            <div className="text-sm">
              <span className="font-medium">{nMap.get(s.requester_nurse_id) ?? `#${s.requester_nurse_id}`}</span>
              <span className="text-gray-500"> ({s.requester_date}) </span>
              <span className="text-gray-400">↔</span>
              <span className="font-medium ml-2">{nMap.get(s.target_nurse_id) ?? `#${s.target_nurse_id}`}</span>
              <span className="text-gray-500"> ({s.target_date})</span>
            </div>
            {s.reason && <p className="text-xs text-gray-600 mt-1">사유: {s.reason}</p>}
            {s.rejected_reason && <p className="text-xs text-red-700 mt-1">거부 사유: {s.rejected_reason}</p>}

            <div className="flex gap-2 mt-3">
              {kind === "inbox" && s.status === "pending" && (
                <>
                  <Button size="sm" onClick={() => patch.mutate({ id: s.id, action: "accept" })}>수락</Button>
                  <Button size="sm" variant="secondary" onClick={() => patch.mutate({ id: s.id, action: "reject" })}>거부</Button>
                </>
              )}
              {kind === "sent" && (s.status === "pending" || s.status === "b_accepted") && (
                <Button size="sm" variant="ghost" onClick={() => cancel.mutate(s.id)}>취소</Button>
              )}
              {kind === "approval" && s.status === "b_accepted" && (
                <>
                  <Button size="sm" onClick={() => patch.mutate({ id: s.id, action: "approve" })}>승인 (X-03 검증)</Button>
                </>
              )}
            </div>
          </article>
        ))
      )}
    </section>
  );

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">근무 교환 (Swap)</h1>
          <p className="text-sm text-gray-600 mt-1">상태머신: pending → b_accepted → approved | rejected | cancelled</p>
        </div>
        <Button onClick={() => setCreating(true)}>+ 새 요청</Button>
      </div>

      {patch.error && (<ErrorBox err={patch.error as ApiException} />)}

      <CardList title="받은 요청" items={inbox.data} kind="inbox" />
      <CardList title="보낸 요청" items={sent.data} kind="sent" />
      {isHead && <CardList title="승인 대기 (수간호사)" items={pendingApproval.data} kind="approval" />}

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

// ----- swap 요청 생성 모달 -----

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

  const myShifts = useMemo(() => {
    return (cells.data ?? []).filter((c) => c.nurse_id === me.nid);
  }, [cells.data, me.nid]);
  const targetShifts = useMemo(() => {
    if (!targetNurseId) return [];
    return (cells.data ?? []).filter((c) => c.nurse_id === targetNurseId);
  }, [cells.data, targetNurseId]);

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
    !!schedule.data?.id &&
    !!targetNurseId &&
    !!requesterDate &&
    !!targetDate;

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
          {schedule.error && <p className="text-xs text-red-600 mt-1">해당 월의 schedule이 아직 없습니다.</p>}
          {schedule.data?.status && schedule.data.status !== "confirmed" && (
            <p className="text-xs text-yellow-700 mt-1">
              status={schedule.data.status} — 보통 confirmed 상태에서 swap합니다. 진행은 가능.
            </p>
          )}
        </label>

        <div className="grid sm:grid-cols-2 gap-4">
          <div className="border rounded-lg p-3 bg-gray-50">
            <div className="font-semibold mb-2">내 셀</div>
            {myShifts.length === 0 ? (
              <p className="text-xs text-gray-400">셀 없음</p>
            ) : (
              <div className="grid grid-cols-7 gap-1 max-h-48 overflow-y-auto">
                {myShifts.map((c) => (
                  <button
                    key={c.id}
                    onClick={() => setRequesterDate(c.date)}
                    className={clsx(
                      "rounded text-[10px] py-1 px-0.5 transition",
                      SHIFT_BG[c.shift as ShiftCode],
                      SHIFT_TEXT[c.shift as ShiftCode],
                      requesterDate === c.date && "ring-2 ring-blue-500",
                    )}
                    title={`${c.date} ${c.shift}`}
                  >
                    {Number(c.date.slice(-2))}
                  </button>
                ))}
              </div>
            )}
            <div className="text-xs mt-2 text-gray-600">
              내 날짜: <span className="font-mono">{requesterDate || "-"}</span>
            </div>
          </div>

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
            {targetNurseId && targetShifts.length === 0 ? (
              <p className="text-xs text-gray-400">해당 월에 상대 셀 없음</p>
            ) : (
              <div className="grid grid-cols-7 gap-1 max-h-48 overflow-y-auto">
                {targetShifts.map((c) => (
                  <button
                    key={c.id}
                    onClick={() => setTargetDate(c.date)}
                    className={clsx(
                      "rounded text-[10px] py-1 px-0.5 transition",
                      SHIFT_BG[c.shift as ShiftCode],
                      SHIFT_TEXT[c.shift as ShiftCode],
                      targetDate === c.date && "ring-2 ring-blue-500",
                    )}
                    title={`${c.date} ${c.shift}`}
                  >
                    {Number(c.date.slice(-2))}
                  </button>
                ))}
              </div>
            )}
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
          <Button variant="ghost" onClick={onClose}>취소</Button>
          <Button onClick={() => create.mutate()} disabled={!canSubmit || create.isPending}>
            {create.isPending ? "요청 중…" : "요청 보내기"}
          </Button>
        </div>
      </div>
    </Modal>
  );
}

function ErrorBox({ err }: { err: ApiException }) {
  return (
    <div className="bg-red-50 border border-red-200 rounded px-3 py-2 text-sm">
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
