"use client";
import { useState, useMemo } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch, ApiException } from "@/lib/api";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import type { Nurse, Subject } from "@/lib/types";

interface Assignment {
  id: number; nurse_id: number; year_month: string;
  assigned_by_nurse_id?: number | null; reason?: string | null; created_at: string;
}

function thisMonth() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

function next12Months(from: string): string[] {
  const [y, m] = from.split("-").map(Number);
  const arr: string[] = [];
  for (let i = 0; i < 12; i++) {
    const t = new Date(y, m - 1 + i, 1);
    arr.push(`${t.getFullYear()}-${String(t.getMonth() + 1).padStart(2, "0")}`);
  }
  return arr;
}

export default function NightKeepersPage() {
  const qc = useQueryClient();
  const { data: me } = useQuery<Subject>({ queryKey: ["me"], queryFn: () => apiFetch<Subject>("/auth/me") });
  const isHead = me?.rl === "head_nurse";
  const nurses = useQuery<Nurse[]>({ queryKey: ["nurses"], queryFn: () => apiFetch<Nurse[]>("/nurses") });

  const [startYM] = useState(thisMonth());
  const months = useMemo(() => next12Months(startYM), [startYM]);

  // 12개월 각각 별도 쿼리 (병렬)
  const monthQueries = months.map((ym) =>
    useQuery<Assignment[]>({
      queryKey: ["night-keepers", ym],
      queryFn: () => apiFetch<Assignment[]>("/night-keepers", { search: { ym } }),
    }),
  );

  const [adding, setAdding] = useState<{ ym: string } | null>(null);

  const create = useMutation<Assignment, ApiException, { nurse_id: number; year_month: string; reason?: string }>({
    mutationFn: (body) => apiFetch<Assignment>("/night-keepers", { method: "POST", body }),
    onSuccess: () => {
      months.forEach((ym) => qc.invalidateQueries({ queryKey: ["night-keepers", ym] }));
      setAdding(null);
    },
  });
  const del = useMutation({
    mutationFn: (id: number) => apiFetch(`/night-keepers/${id}`, { method: "DELETE" }),
    onSuccess: () => months.forEach((ym) => qc.invalidateQueries({ queryKey: ["night-keepers", ym] })),
  });

  if (nurses.isLoading) return <Spinner size={6} />;
  const nurseMap = new Map((nurses.data ?? []).map((n) => [n.id, n]));

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-bold">나이트킵 (K-NN)</h1>
        <p className="text-sm text-gray-600 mt-1">
          K-01: 지정 시 N/O만 · K-02: 3달 연속 금지 · K-03: 표준 2달 · K-04: cooldown 3달 · K-05: 고정 패턴과 충돌 금지
        </p>
      </div>

      <div className="bg-white border rounded-xl overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 text-xs text-gray-600">
            <tr>
              <th className="text-left px-3 py-2">월</th>
              <th className="text-left px-3 py-2">지정된 간호사</th>
              {isHead && <th className="px-3 py-2"></th>}
            </tr>
          </thead>
          <tbody>
            {months.map((ym, idx) => {
              const list = monthQueries[idx].data ?? [];
              return (
                <tr key={ym} className="border-t">
                  <td className="px-3 py-2 font-mono">{ym}</td>
                  <td className="px-3 py-2">
                    {list.length === 0 ? (
                      <span className="text-gray-400 text-xs">없음</span>
                    ) : (
                      <div className="flex flex-wrap gap-1.5">
                        {list.map((a) => {
                          const n = nurseMap.get(a.nurse_id);
                          return (
                            <Badge key={a.id} variant="info" className="flex items-center gap-1">
                              <span>🌙 {n?.name ?? `#${a.nurse_id}`}</span>
                              {isHead && (
                                <button
                                  className="text-blue-600 hover:text-blue-800 -mr-1 ml-1"
                                  onClick={() => del.mutate(a.id)}
                                  title="해제"
                                >
                                  ×
                                </button>
                              )}
                            </Badge>
                          );
                        })}
                      </div>
                    )}
                  </td>
                  {isHead && (
                    <td className="px-3 py-2 text-right">
                      <Button variant="ghost" size="sm" onClick={() => setAdding({ ym })}>+ 지정</Button>
                    </td>
                  )}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {adding && (
        <NkAddModal
          ym={adding.ym}
          nurses={(nurses.data ?? []).filter((n) => n.active)}
          onClose={() => setAdding(null)}
          onSubmit={(nurseId, reason) =>
            create.mutate({ nurse_id: nurseId, year_month: adding.ym, reason: reason || undefined })
          }
          error={create.error}
          submitting={create.isPending}
        />
      )}
    </div>
  );
}

function NkAddModal({ ym, nurses, onClose, onSubmit, error, submitting }: {
  ym: string; nurses: Nurse[]; onClose: () => void;
  onSubmit: (nurseId: number, reason: string) => void;
  error: ApiException | null; submitting: boolean;
}) {
  const [nurseId, setNurseId] = useState<number | "">("");
  const [reason, setReason] = useState("");
  return (
    <Modal open onClose={onClose} title={`나이트킵 지정 — ${ym}`} size="sm">
      <div className="space-y-3 text-sm">
        <label className="block">
          <span className="block text-xs text-gray-600 mb-1">간호사</span>
          <select className="input" value={nurseId} onChange={(e) => setNurseId(e.target.value ? Number(e.target.value) : "")}>
            <option value="">선택…</option>
            {nurses.map((n) => (
              <option key={n.id} value={n.id} disabled={!!n.fixed_shift_pattern}>
                {n.name}{n.fixed_shift_pattern ? ` (고정 ${n.fixed_shift_pattern} — K-05)` : ""}
              </option>
            ))}
          </select>
        </label>
        <label className="block">
          <span className="block text-xs text-gray-600 mb-1">사유 (선택)</span>
          <input className="input" value={reason} onChange={(e) => setReason(e.target.value)} />
        </label>

        {error && (
          <div className="bg-red-50 border border-red-200 rounded px-3 py-2 text-sm">
            <div className="font-semibold text-red-900">
              {error.body.rule_id ? `[${error.body.rule_id}] 차단` : "오류"}
            </div>
            <div className="text-red-800 mt-0.5">{error.body.message}</div>
          </div>
        )}

        <div className="flex justify-end gap-2 pt-2">
          <Button variant="ghost" onClick={onClose}>취소</Button>
          <Button onClick={() => nurseId && onSubmit(nurseId, reason)} disabled={!nurseId || submitting}>
            {submitting ? "저장 중…" : "지정"}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
