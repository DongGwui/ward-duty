"use client";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { Spinner } from "@/components/ui/Spinner";
import type { Level, Subject } from "@/lib/types";

export default function LevelsPage() {
  const qc = useQueryClient();
  const { data: me } = useQuery<Subject>({ queryKey: ["me"], queryFn: () => apiFetch<Subject>("/auth/me") });
  const isHead = me?.rl === "head_nurse";
  const levels = useQuery<Level[]>({ queryKey: ["levels"], queryFn: () => apiFetch<Level[]>("/levels") });
  const [editing, setEditing] = useState<Level | "new" | null>(null);

  const create = useMutation({
    mutationFn: (body: Partial<Level>) => apiFetch<Level>("/levels", { method: "POST", body }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["levels"] }); setEditing(null); },
  });
  const update = useMutation({
    mutationFn: ({ code, body }: { code: string; body: Partial<Level> }) =>
      apiFetch<Level>(`/levels/${code}`, { method: "PATCH", body }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["levels"] }); setEditing(null); },
  });
  const del = useMutation({
    mutationFn: (code: string) => apiFetch(`/levels/${code}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["levels"] }),
  });

  if (levels.isLoading) return <Spinner size={6} />;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">연차 등급</h1>
          <p className="text-sm text-gray-600 mt-1">H-12 · S-10 · G-04 — 시프트당 등급별 최소 인원과 배정 가중치를 정의</p>
        </div>
        {isHead && <Button onClick={() => setEditing("new")}>+ 등급 추가</Button>}
      </div>

      <div className="bg-white border rounded-xl overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 text-xs uppercase text-gray-600">
            <tr>
              <th className="text-left px-3 py-2">code</th>
              <th className="text-left px-3 py-2">표시명</th>
              <th className="text-right px-3 py-2">개월(min~max)</th>
              <th className="text-right px-3 py-2">min_d</th>
              <th className="text-right px-3 py-2">min_e</th>
              <th className="text-right px-3 py-2">min_n</th>
              <th className="text-right px-3 py-2">w_d</th>
              <th className="text-right px-3 py-2">w_e</th>
              <th className="text-right px-3 py-2">w_n</th>
              <th className="text-right px-3 py-2">정렬</th>
              {isHead && <th className="px-3 py-2"></th>}
            </tr>
          </thead>
          <tbody>
            {levels.data?.map((l) => (
              <tr key={l.code} className="border-t hover:bg-gray-50">
                <td className="px-3 py-2 font-mono">{l.code}</td>
                <td className="px-3 py-2">{l.display_name}</td>
                <td className="px-3 py-2 text-right text-xs text-gray-600">{l.min_months}~{l.max_months ?? "∞"}</td>
                <td className="px-3 py-2 text-right">{l.min_d}</td>
                <td className="px-3 py-2 text-right">{l.min_e}</td>
                <td className="px-3 py-2 text-right">{l.min_n}</td>
                <td className="px-3 py-2 text-right text-gray-600">{l.weight_d_assignment}</td>
                <td className="px-3 py-2 text-right text-gray-600">{l.weight_e_assignment}</td>
                <td className="px-3 py-2 text-right text-gray-600">{l.weight_n_assignment}</td>
                <td className="px-3 py-2 text-right text-xs text-gray-500">{l.sort_order}</td>
                {isHead && (
                  <td className="px-3 py-2 text-right whitespace-nowrap">
                    <Button variant="ghost" size="sm" onClick={() => setEditing(l)}>편집</Button>
                    <Button variant="ghost" size="sm" onClick={() => { if (confirm(`${l.code} 삭제?`)) del.mutate(l.code); }}>삭제</Button>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {del.error && (
        <div className="bg-red-50 border border-red-200 rounded px-3 py-2 text-sm text-red-800">
          삭제 실패 — 소속 간호사가 있는 등급은 삭제할 수 없습니다.
        </div>
      )}

      {editing && (
        <LevelEditor
          level={editing === "new" ? null : editing}
          onClose={() => setEditing(null)}
          onSubmit={(body) => {
            if (editing === "new") create.mutate(body);
            else update.mutate({ code: editing.code, body });
          }}
          submitting={create.isPending || update.isPending}
        />
      )}
    </div>
  );
}

function LevelEditor({ level, onClose, onSubmit, submitting }: {
  level: Level | null; onClose: () => void;
  onSubmit: (body: Partial<Level>) => void; submitting: boolean;
}) {
  const isNew = !level;
  const [form, setForm] = useState<Partial<Level>>(() => level ?? {
    code: "", display_name: "", min_months: 0, max_months: null,
    min_d: 0, min_e: 0, min_n: 0,
    weight_coverage: 1, weight_d_assignment: 0, weight_e_assignment: 0, weight_n_assignment: 0,
    sort_order: 0,
  });

  const set = (k: keyof Level, v: unknown) => setForm({ ...form, [k]: v });
  const num = (k: keyof Level, v: string) => set(k, v === "" ? null : Number(v));

  return (
    <Modal open onClose={onClose} title={isNew ? "등급 추가" : `편집 — ${level?.code}`} size="lg">
      <div className="grid sm:grid-cols-2 gap-3 text-sm">
        <Field label="code (예: L1, SR)"><input className="input" value={form.code ?? ""} onChange={(e) => set("code", e.target.value)} disabled={!isNew} /></Field>
        <Field label="표시명"><input className="input" value={form.display_name ?? ""} onChange={(e) => set("display_name", e.target.value)} /></Field>
        <Field label="min_months"><input className="input" type="number" value={String(form.min_months ?? 0)} onChange={(e) => num("min_months", e.target.value)} /></Field>
        <Field label="max_months (빈칸=무제한)"><input className="input" type="number" value={form.max_months === null || form.max_months === undefined ? "" : String(form.max_months)} onChange={(e) => num("max_months", e.target.value)} /></Field>
        <Field label="min_d"><input className="input" type="number" value={String(form.min_d ?? 0)} onChange={(e) => num("min_d", e.target.value)} /></Field>
        <Field label="min_e"><input className="input" type="number" value={String(form.min_e ?? 0)} onChange={(e) => num("min_e", e.target.value)} /></Field>
        <Field label="min_n"><input className="input" type="number" value={String(form.min_n ?? 0)} onChange={(e) => num("min_n", e.target.value)} /></Field>
        <Field label="sort_order"><input className="input" type="number" value={String(form.sort_order ?? 0)} onChange={(e) => num("sort_order", e.target.value)} /></Field>
        <Field label="weight_d_assignment"><input className="input" type="number" value={String(form.weight_d_assignment ?? 0)} onChange={(e) => num("weight_d_assignment", e.target.value)} /></Field>
        <Field label="weight_e_assignment"><input className="input" type="number" value={String(form.weight_e_assignment ?? 0)} onChange={(e) => num("weight_e_assignment", e.target.value)} /></Field>
        <Field label="weight_n_assignment"><input className="input" type="number" value={String(form.weight_n_assignment ?? 0)} onChange={(e) => num("weight_n_assignment", e.target.value)} /></Field>
        <Field label="weight_coverage"><input className="input" type="number" value={String(form.weight_coverage ?? 1)} onChange={(e) => num("weight_coverage", e.target.value)} /></Field>
      </div>
      <div className="flex justify-end gap-2 pt-4">
        <Button variant="ghost" onClick={onClose}>취소</Button>
        <Button onClick={() => onSubmit(form)} disabled={submitting}>{submitting ? "저장 중…" : "저장"}</Button>
      </div>
    </Modal>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="block text-xs text-gray-600 mb-1 font-mono">{label}</span>
      {children}
    </label>
  );
}
