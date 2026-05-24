"use client";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import type { Nurse, Level, Subject, FixedPattern } from "@/lib/types";

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

  if (nurses.isLoading) return <Spinner size={6} />;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">간호사 명단</h1>
        {isHead && <Button onClick={() => setEditing("new")}>+ 추가</Button>}
      </div>

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
                <td className="px-3 py-2 text-gray-600">{n.email}</td>
                <td className="px-3 py-2">
                  {n.role === "head_nurse" ? <Badge variant="info">수간호사</Badge> : "팀원"}
                </td>
                <td className="px-3 py-2 text-gray-600">{n.hire_date ?? "-"}</td>
                <td className="px-3 py-2">
                  <span className="font-mono text-xs">{n.resolved_level ?? "-"}</span>
                  {n.experience_level_override && <span className="text-xs text-gray-500 ml-1">(수동)</span>}
                </td>
                <td className="px-3 py-2 text-xs">{n.fixed_shift_pattern ?? "-"}</td>
                <td className="px-3 py-2">{n.active ? "✅" : <span className="text-gray-400">비활성</span>}</td>
                {isHead && (
                  <td className="px-3 py-2 text-right">
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
        <Field label="이메일 (Google 화이트리스트)">
          <input className="input" type="email" value={form.email ?? ""} onChange={(e) => setForm({ ...form, email: e.target.value })} disabled={!isNew} />
          {!isNew && <p className="text-xs text-gray-500 mt-1">이메일은 수정 불가 — 신규 추가로 처리하세요.</p>}
        </Field>
        <Field label="역할">
          <select className="input" value={form.role ?? "nurse"} onChange={(e) => setForm({ ...form, role: e.target.value as Nurse["role"] })}>
            <option value="nurse">팀원</option>
            <option value="head_nurse">수간호사</option>
          </select>
        </Field>
        <Field label="입사일 (G-04 자동 분류 기준)">
          <input className="input" type="date" value={form.hire_date?.slice(0, 10) ?? ""} onChange={(e) => setForm({ ...form, hire_date: e.target.value || null })} />
        </Field>
        <Field label="등급 수동 지정 (override)">
          <select className="input" value={form.experience_level_override ?? ""} onChange={(e) => setForm({ ...form, experience_level_override: e.target.value || null })}>
            <option value="">자동 (hire_date 기반)</option>
            {levels.map((l) => <option key={l.code} value={l.code}>{l.code} · {l.display_name}</option>)}
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
