"use client";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import type { Nurse, Level, Subject, FixedPattern } from "@/lib/types";

interface PendingAccount {
  email: string;
  google_sub: string;
  name?: string;
  picture?: string;
  created_at: string;
}

const FIXED_OPTIONS: { value: FixedPattern | ""; label: string }[] = [
  { value: "", label: "일반 로테이션" },
  { value: "D_ONLY", label: "D 전담" },
  { value: "E_ONLY", label: "E 전담" },
  { value: "N_ONLY", label: "N 전담" },
  { value: "WEEKDAY_D", label: "평일 D" },
  { value: "WEEKDAY_E", label: "평일 E" },
];

export default function NursesPage() {
  const qc = useQueryClient();
  const { data: me } = useQuery<Subject>({ queryKey: ["me"], queryFn: () => apiFetch<Subject>("/auth/me") });
  const isHead = me?.rl === "head_nurse";
  const myId = me?.nid;
  const nurses = useQuery<Nurse[]>({
    queryKey: ["nurses", { include_inactive: true }],
    queryFn: () => apiFetch<Nurse[]>("/nurses", { search: { include_inactive: 1 } }),
  });
  const levels = useQuery<Level[]>({ queryKey: ["levels"], queryFn: () => apiFetch<Level[]>("/levels") });

  const [editing, setEditing] = useState<Nurse | "new" | null>(null);

  const create = useMutation({
    mutationFn: (body: Partial<Nurse>) => apiFetch<Nurse>("/nurses", { method: "POST", body }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["nurses"] }); setEditing(null); },
  });
  const update = useMutation({
    mutationFn: ({ id, body }: { id: number; body: Partial<Nurse> }) =>
      apiFetch<Nurse>(`/nurses/${id}`, { method: "PATCH", body }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["nurses"] }); setEditing(null); },
  });

  // 인라인 빠른 수정용 (모달 닫기 안 함)
  const inlinePatch = useMutation({
    mutationFn: ({ id, body }: { id: number; body: Partial<Nurse> }) =>
      apiFetch<Nurse>(`/nurses/${id}`, { method: "PATCH", body }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["nurses"] }),
  });
  // 어떤 row가 inline 저장 중인지 시각화 (작은 spinner)
  const [savingId, setSavingId] = useState<number | null>(null);
  function patchInline(id: number, body: Partial<Nurse>) {
    setSavingId(id);
    inlinePatch.mutate(
      { id, body },
      { onSettled: () => setSavingId((v) => (v === id ? null : v)) },
    );
  }

  // Stage 2: pending OAuth 계정 (매니저만)
  const pending = useQuery<PendingAccount[]>({
    queryKey: ["pending-accounts"],
    queryFn: () => apiFetch<PendingAccount[]>("/pending-accounts"),
    enabled: isHead,
    refetchInterval: 15_000, // 15초마다 자동 폴링
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
    mutationFn: (email: string) => apiFetch(`/pending-accounts/${encodeURIComponent(email)}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["pending-accounts"] }),
  });

  if (nurses.isLoading) return <Spinner size={6} />;

  return (
    <div className="space-y-4">
      {/* Stage 2: pending 매칭 섹션 */}
      {isHead && (pending.data ?? []).length > 0 && (
        <section className="bg-yellow-50 border border-yellow-300 rounded-xl overflow-hidden">
          <header className="px-5 py-3 border-b border-yellow-200 bg-yellow-100/50">
            <h2 className="font-semibold text-yellow-900 flex items-center gap-2">
              🔔 계정 연결 대기
              <Badge variant="warning">{pending.data!.length}</Badge>
            </h2>
            <p className="text-xs text-yellow-800 mt-1">
              Google 로그인을 시도했지만 명단에 없는 사용자입니다. 어느 nurse 행에 연결할지 선택하세요.
            </p>
          </header>
          <div className="p-4 space-y-3">
            {pending.data!.map((p) => (
              <PendingCard
                key={p.email}
                p={p}
                nurses={(nurses.data ?? []).filter((n) => !n.email)}
                onLink={(nurseId) => linkAccount.mutate({ nurseId, email: p.email })}
                onDismiss={() => dismissPending.mutate(p.email)}
                submitting={linkAccount.isPending || dismissPending.isPending}
              />
            ))}
          </div>
        </section>
      )}

      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">간호사 명단</h1>
          {isHead && (
            <p className="text-xs text-gray-500 mt-1">
              테이블의 등급·고정 패턴·활성 셀을 클릭하면 즉시 저장됩니다. 이름·이메일·입사일 등은 "편집" 버튼을 이용하세요.
            </p>
          )}
        </div>
        {isHead && <Button onClick={() => setEditing("new")}>+ 추가</Button>}
      </div>

      {inlinePatch.error && (
        <div className="bg-red-50 border border-red-200 rounded px-3 py-2 text-sm text-red-800">
          저장 실패: {(inlinePatch.error as Error).message}
        </div>
      )}

      <div className="bg-white border rounded-xl overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 text-xs uppercase text-gray-600">
            <tr>
              <th className="text-left px-3 py-2">이름</th>
              <th className="text-left px-3 py-2">이메일</th>
              <th className="text-left px-3 py-2">역할</th>
              <th className="text-left px-3 py-2">입사일</th>
              <th className="text-left px-3 py-2">등급</th>
              <th className="text-left px-3 py-2">고정 패턴</th>
              <th className="text-left px-3 py-2">활성</th>
              {isHead && <th className="px-3 py-2"></th>}
            </tr>
          </thead>
          <tbody>
            {nurses.data?.map((n) => {
              const isMe = n.id === myId;
              return (
              <tr key={n.id} className={isMe ? "border-t bg-blue-50/70 hover:bg-blue-50" : "border-t hover:bg-gray-50"}>
                <td className="px-3 py-2 font-medium">
                  {n.name}
                  {isMe && <span className="ml-1.5 text-[10px] bg-blue-600 text-white rounded px-1 py-0.5">나</span>}
                </td>
                <td className="px-3 py-2 text-gray-600">
                  {n.email ? (
                    n.email
                  ) : (
                    <span className="text-xs text-gray-400 italic">계정 미연결</span>
                  )}
                </td>
                <td className="px-3 py-2">
                  {n.role === "head_nurse" ? <Badge variant="info">매니저</Badge> : "팀원"}
                </td>
                <td className="px-3 py-2 text-gray-600">{n.hire_date?.slice(0, 10) ?? "-"}</td>

                {/* 등급 — inline select */}
                <td className="px-3 py-2">
                  {isHead ? (
                    <select
                      className="text-xs font-mono rounded border border-gray-200 px-1.5 py-0.5 bg-white hover:border-blue-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                      value={n.experience_level_override ?? ""}
                      onChange={(e) =>
                        patchInline(n.id, { experience_level_override: e.target.value || null })
                      }
                      disabled={savingId === n.id}
                    >
                      <option value="">— 미지정</option>
                      {(levels.data ?? []).map((l) => (
                        <option key={l.code} value={l.code}>
                          {l.code} · {l.display_name}
                        </option>
                      ))}
                    </select>
                  ) : (
                    <span className="font-mono text-xs">
                      {n.resolved_level ?? "-"}
                      {!n.experience_level_override && (
                        <span className="text-gray-400 ml-1">(미지정)</span>
                      )}
                    </span>
                  )}
                </td>

                {/* 고정 패턴 — inline select */}
                <td className="px-3 py-2">
                  {isHead ? (
                    <select
                      className="text-xs rounded border border-gray-200 px-1.5 py-0.5 bg-white hover:border-blue-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                      value={n.fixed_shift_pattern ?? ""}
                      onChange={(e) =>
                        patchInline(n.id, {
                          fixed_shift_pattern: (e.target.value || null) as FixedPattern | null,
                        })
                      }
                      disabled={savingId === n.id}
                    >
                      {FIXED_OPTIONS.map((o) => (
                        <option key={o.value} value={o.value}>{o.label}</option>
                      ))}
                    </select>
                  ) : (
                    <span className="text-xs">{n.fixed_shift_pattern ?? "-"}</span>
                  )}
                </td>

                {/* 활성 — 클릭 토글 */}
                <td className="px-3 py-2">
                  {isHead ? (
                    <button
                      onClick={() => patchInline(n.id, { active: !n.active })}
                      disabled={savingId === n.id || isMe} // 본인 비활성화 방지
                      title={isMe ? "본인 계정은 비활성화 불가" : n.active ? "비활성화" : "다시 활성화"}
                      className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs hover:bg-gray-100 disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                      {n.active ? (
                        <span className="text-green-700">✅ 활성</span>
                      ) : (
                        <span className="text-gray-400">⏸ 비활성</span>
                      )}
                    </button>
                  ) : n.active ? "✅" : <span className="text-gray-400">비활성</span>}
                </td>

                {isHead && (
                  <td className="px-3 py-2 text-right whitespace-nowrap">
                    {savingId === n.id && <span className="text-[11px] text-gray-400 mr-2">저장 중…</span>}
                    <Button variant="ghost" size="sm" onClick={() => setEditing(n)}>편집</Button>
                  </td>
                )}
              </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {editing && (
        <NurseEditor
          nurse={editing === "new" ? null : editing}
          levels={levels.data ?? []}
          onClose={() => setEditing(null)}
          onSubmit={(body) => {
            if (editing === "new") create.mutate(body);
            else update.mutate({ id: editing.id, body });
          }}
          submitting={create.isPending || update.isPending}
        />
      )}
    </div>
  );
}

function NurseEditor({ nurse, levels, onClose, onSubmit, submitting }: {
  nurse: Nurse | null; levels: Level[]; onClose: () => void;
  onSubmit: (body: Partial<Nurse>) => void; submitting: boolean;
}) {
  const isNew = !nurse;
  const [form, setForm] = useState<Partial<Nurse>>(() => ({
    name: nurse?.name ?? "",
    email: nurse?.email ?? "",
    role: nurse?.role ?? "nurse",
    hire_date: nurse?.hire_date ?? null,
    experience_level_override: nurse?.experience_level_override ?? null,
    fixed_shift_pattern: nurse?.fixed_shift_pattern ?? null,
    active: nurse?.active ?? true,
  }));

  return (
    <Modal open onClose={onClose} title={isNew ? "간호사 추가" : `편집 — ${nurse?.name}`} size="md">
      <div className="space-y-3 text-sm">
        <Field label="이름">
          <input className="input" value={form.name ?? ""} onChange={(e) => setForm({ ...form, name: e.target.value })} disabled={!isNew && false} />
        </Field>
        <Field label="이메일 (선택)">
          <input
            className="input"
            type="email"
            value={form.email ?? ""}
            placeholder="비워두면 계정 없이 명단만 추가"
            onChange={(e) => setForm({ ...form, email: e.target.value || null })}
            disabled={!isNew}
          />
          {isNew ? (
            <p className="text-xs text-gray-500 mt-1">
              비워두면 듀티 명단에만 추가됩니다 (계정 미연결). 이후 Google 로그인 시 매니저가 매칭해 줍니다.
            </p>
          ) : (
            <p className="text-xs text-gray-500 mt-1">이메일은 수정 불가 — 신규 추가로 처리하세요.</p>
          )}
        </Field>
        <Field label="역할">
          <select className="input" value={form.role ?? "nurse"} onChange={(e) => setForm({ ...form, role: e.target.value as Nurse["role"] })}>
            <option value="nurse">팀원</option>
            <option value="head_nurse">매니저</option>
          </select>
        </Field>
        <Field label="입사일 (참고용)">
          <input className="input" type="date" value={form.hire_date?.slice(0, 10) ?? ""} onChange={(e) => setForm({ ...form, hire_date: e.target.value || null })} />
          <p className="text-[11px] text-gray-500 mt-1">v0.5: 등급은 입사일로 자동 분류되지 않습니다. 아래에서 직접 지정하세요.</p>
        </Field>
        <Field label="연차 등급">
          <select
            className="input"
            value={form.experience_level_override ?? ""}
            onChange={(e) => setForm({ ...form, experience_level_override: e.target.value || null })}
          >
            <option value="">미지정 (가장 낮은 등급으로 처리)</option>
            {levels.map((l) => (
              <option key={l.code} value={l.code}>
                {l.code} · {l.display_name}
              </option>
            ))}
          </select>
        </Field>
        <Field label="고정 시프트 패턴 (H-11)">
          <select className="input" value={form.fixed_shift_pattern ?? ""} onChange={(e) => setForm({ ...form, fixed_shift_pattern: (e.target.value || null) as FixedPattern | null })}>
            {FIXED_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
          </select>
        </Field>
        {!isNew && (
          <label className="flex items-center gap-2">
            <input type="checkbox" checked={form.active ?? true} onChange={(e) => setForm({ ...form, active: e.target.checked })} />
            <span>활성</span>
          </label>
        )}
        <div className="flex justify-end gap-2 pt-3">
          <Button variant="ghost" onClick={onClose}>취소</Button>
          <Button onClick={() => onSubmit(form)} disabled={submitting}>{submitting ? "저장 중…" : "저장"}</Button>
        </div>
      </div>
    </Modal>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="block text-xs text-gray-600 mb-1">{label}</span>
      {children}
    </label>
  );
}

// ============================================================
// Stage 2: PendingCard — pending OAuth 계정 매칭
// ============================================================

function PendingCard({
  p,
  nurses,
  onLink,
  onDismiss,
  submitting,
}: {
  p: PendingAccount;
  nurses: Nurse[]; // email이 비어있는 (= 미연결) nurse 후보
  onLink: (nurseId: number) => void;
  onDismiss: () => void;
  submitting: boolean;
}) {
  const [selectedNurseId, setSelectedNurseId] = useState<number | "">("");
  return (
    <article className="bg-white border border-yellow-200 rounded-lg p-4">
      <div className="flex items-center justify-between flex-wrap gap-2 mb-3">
        <div className="flex items-center gap-3">
          {p.picture && (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={p.picture} alt="" className="w-8 h-8 rounded-full" />
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
            <option disabled value="">미연결 nurse 없음 — 먼저 + 추가</option>
          ) : (
            nurses.map((n) => (
              <option key={n.id} value={n.id}>
                {n.name} {n.resolved_level ? `(${n.resolved_level})` : ""}
              </option>
            ))
          )}
        </select>
        <Button
          size="sm"
          onClick={() => selectedNurseId && onLink(selectedNurseId)}
          disabled={!selectedNurseId || submitting}
        >
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
