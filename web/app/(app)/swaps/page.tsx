"use client";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch, ApiException } from "@/lib/api";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import type { Subject, Nurse, SwapStatus } from "@/lib/types";

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
      <div>
        <h1 className="text-2xl font-bold">근무 교환 (Swap)</h1>
        <p className="text-sm text-gray-600 mt-1">상태머신: pending → b_accepted → approved | rejected_by_b | rejected_by_head | cancelled</p>
      </div>

      {patch.error && (
        <ErrorBox err={patch.error as ApiException} />
      )}

      <CardList title="받은 요청" items={inbox.data} kind="inbox" />
      <CardList title="보낸 요청" items={sent.data} kind="sent" />
      {isHead && <CardList title="승인 대기 (수간호사)" items={pendingApproval.data} kind="approval" />}
    </div>
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
