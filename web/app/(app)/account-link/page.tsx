"use client";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import type { Nurse, Subject } from "@/lib/types";

interface PendingAccount {
  email: string;
  google_sub: string;
  name?: string;
  picture?: string;
  created_at: string;
}

export default function AccountLinkPage() {
  const qc = useQueryClient();
  const { data: me } = useQuery<Subject>({
    queryKey: ["me"],
    queryFn: () => apiFetch<Subject>("/auth/me"),
  });
  const isHead = me?.rl === "head_nurse";

  const pending = useQuery<PendingAccount[]>({
    queryKey: ["pending-accounts"],
    queryFn: () => apiFetch<PendingAccount[]>("/pending-accounts"),
    enabled: isHead,
    refetchInterval: 15_000,
  });

  const nurses = useQuery<Nurse[]>({
    queryKey: ["nurses", { include_inactive: true }],
    queryFn: () => apiFetch<Nurse[]>("/nurses", { search: { include_inactive: 1 } }),
    enabled: isHead,
  });

  const linkAccount = useMutation({
    mutationFn: ({ nurseId, email }: { nurseId: number; email: string }) =>
      apiFetch(`/nurses/${nurseId}/link-account`, { method: "POST", body: { email } }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["pending-accounts"] });
      qc.invalidateQueries({ queryKey: ["nurses"] });
    },
  });
  const dismissPending = useMutation({
    mutationFn: (email: string) =>
      apiFetch(`/pending-accounts/${encodeURIComponent(email)}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["pending-accounts"] }),
  });

  if (!isHead) {
    return <p className="text-sm text-gray-500">매니저만 접근 가능합니다.</p>;
  }
  if (pending.isLoading || nurses.isLoading) return <Spinner size={6} />;

  const items = pending.data ?? [];
  const unlinkedNurses = (nurses.data ?? []).filter((n) => !n.email);

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-bold flex items-center gap-2">
          계정 연결 대기
          {items.length > 0 && <Badge variant="warning">{items.length}</Badge>}
        </h1>
        <p className="text-sm text-gray-600 mt-1">
          Google 로그인을 시도했지만 명단에 없는 사용자입니다. 어느 nurse 행에 연결할지
          선택해 주세요. <span className="text-gray-400">미연결 nurse가 부족하면 먼저 명단에 추가하세요.</span>
        </p>
      </div>

      {items.length === 0 ? (
        <div className="bg-white border rounded-xl py-16 text-center">
          <div className="text-4xl opacity-30 mb-2">📭</div>
          <p className="text-sm text-gray-500">대기 중인 계정 연결 요청이 없습니다.</p>
        </div>
      ) : (
        <div className="space-y-3">
          {items.map((p) => (
            <PendingCard
              key={p.email}
              p={p}
              nurses={unlinkedNurses}
              onLink={(nurseId) => linkAccount.mutate({ nurseId, email: p.email })}
              onDismiss={() => dismissPending.mutate(p.email)}
              submitting={linkAccount.isPending || dismissPending.isPending}
            />
          ))}
        </div>
      )}

      {linkAccount.error && (
        <div className="bg-red-50 border border-red-200 rounded px-3 py-2 text-sm text-red-800">
          연결 실패: {(linkAccount.error as Error).message}
        </div>
      )}
    </div>
  );
}

function PendingCard({
  p, nurses, onLink, onDismiss, submitting,
}: {
  p: PendingAccount;
  nurses: Nurse[];
  onLink: (nurseId: number) => void;
  onDismiss: () => void;
  submitting: boolean;
}) {
  const [selectedNurseId, setSelectedNurseId] = useState<number | "">("");
  return (
    <article className="bg-white border rounded-lg p-4">
      <div className="flex items-center justify-between flex-wrap gap-2 mb-3">
        <div className="flex items-center gap-3">
          {p.picture && (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={p.picture} alt="" className="w-10 h-10 rounded-full" />
          )}
          <div>
            <div className="font-medium">{p.name || p.email}</div>
            <div className="text-xs text-gray-500 font-mono">{p.email}</div>
          </div>
        </div>
        <span className="text-[11px] text-gray-400">
          {new Date(p.created_at).toLocaleString()}
        </span>
      </div>

      <div className="grid sm:grid-cols-[1fr_auto_auto] gap-2 items-center">
        <select
          className="input"
          value={selectedNurseId}
          onChange={(e) => setSelectedNurseId(e.target.value ? Number(e.target.value) : "")}
        >
          <option value="">연결할 명단 선택…</option>
          {nurses.length === 0 ? (
            <option disabled value="">미연결 nurse 없음 — /nurses에서 먼저 추가</option>
          ) : (
            nurses.map((n) => (
              <option key={n.id} value={n.id}>
                {n.name} {n.resolved_level ? `(${n.resolved_level})` : ""}
              </option>
            ))
          )}
        </select>
        <Button size="sm" onClick={() => selectedNurseId && onLink(selectedNurseId)} disabled={!selectedNurseId || submitting}>
          연결
        </Button>
        <Button size="sm" variant="ghost" onClick={onDismiss} disabled={submitting}>
          거부
        </Button>
      </div>

      <p className="text-[11px] text-gray-500 mt-2">
        연결 후 사용자가 다시 Google 로그인하면 해당 nurse로 접속할 수 있습니다.
      </p>
    </article>
  );
}
