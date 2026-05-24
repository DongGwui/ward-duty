"use client";
import { useState, useMemo } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { Spinner } from "@/components/ui/Spinner";
import type { Subject, WishType, Nurse } from "@/lib/types";
import clsx from "clsx";

interface Wish {
  id: number; nurse_id: number; date: string; type: WishType; reason?: string | null;
}

const TYPE_EMOJI: Record<WishType, string> = {
  off: "🛌",
  d: "☀️",
  e: "🌆",
  n: "🌙",
  unavailable: "🚫",
};
const TYPE_LABEL: Record<WishType, string> = {
  off: "Off", d: "Day", e: "Evening", n: "Night", unavailable: "불가",
};
const TYPE_BG: Record<WishType, string> = {
  off: "bg-gray-200 text-gray-800",
  d: "bg-blue-100 text-blue-800",
  e: "bg-orange-100 text-orange-800",
  n: "bg-purple-100 text-purple-800",
  unavailable: "bg-red-100 text-red-800",
};

function thisMonth() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

function daysInMonth(ym: string): number {
  const [y, m] = ym.split("-").map(Number);
  return new Date(y, m, 0).getDate();
}

function firstWeekday(ym: string): number {
  const [y, m] = ym.split("-").map(Number);
  return new Date(y, m - 1, 1).getDay(); // 0=Sun
}

export default function WishesPage() {
  const router = useRouter();
  const sp = useSearchParams();
  const { data: me } = useQuery<Subject>({ queryKey: ["me"], queryFn: () => apiFetch<Subject>("/auth/me") });
  const isHead = me?.rl === "head_nurse";

  const ym = sp.get("ym") ?? thisMonth();
  const filterNurse = sp.get("nurse");

  const nurses = useQuery<Nurse[]>({ queryKey: ["nurses"], queryFn: () => apiFetch<Nurse[]>("/nurses") });

  const wishes = useQuery<Wish[]>({
    queryKey: ["wishes", ym, filterNurse],
    queryFn: () => apiFetch<Wish[]>("/wishes", { search: { ym, nurse: filterNurse ?? undefined } }),
  });

  const qc = useQueryClient();
  const upsert = useMutation({
    mutationFn: ({ date, body }: { date: string; body: { type: WishType; reason?: string } }) =>
      apiFetch<Wish>(`/wishes/${date}`, { method: "PUT", body }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["wishes"] }),
  });
  const del = useMutation({
    mutationFn: (date: string) => apiFetch(`/wishes/${date}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["wishes"] }),
  });

  const wishMap = useMemo(() => {
    const m = new Map<string, Wish>();
    for (const w of wishes.data ?? []) m.set(w.date, w);
    return m;
  }, [wishes.data]);

  const days = daysInMonth(ym);
  const skip = firstWeekday(ym);
  const [picker, setPicker] = useState<{ date: string; current?: Wish } | null>(null);

  function changeMonth(delta: number) {
    const [y, m] = ym.split("-").map(Number);
    const t = new Date(y, m - 1 + delta, 1);
    const nym = `${t.getFullYear()}-${String(t.getMonth() + 1).padStart(2, "0")}`;
    router.push(`/wishes?ym=${nym}${filterNurse ? `&nurse=${filterNurse}` : ""}`);
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="sm" onClick={() => changeMonth(-1)}>←</Button>
          <h1 className="text-xl font-bold">{ym} 희망일</h1>
          <Button variant="ghost" size="sm" onClick={() => changeMonth(1)}>→</Button>
        </div>
        {isHead && (
          <select
            className="input max-w-xs"
            value={filterNurse ?? ""}
            onChange={(e) => {
              const v = e.target.value;
              router.push(`/wishes?ym=${ym}${v ? `&nurse=${v}` : ""}`);
            }}
          >
            <option value="">전체 (수간호사 시점)</option>
            {nurses.data?.map((n) => <option key={n.id} value={n.id}>{n.name}</option>)}
          </select>
        )}
      </div>

      <p className="text-xs text-gray-600">
        날짜를 탭하여 희망 시프트(off/d/e/n) 또는 불가일(unavailable)을 등록하세요. unavailable은 hard로 강제됩니다 (H-05).
      </p>

      {wishes.isLoading ? (
        <Spinner size={6} />
      ) : (
        <div className="bg-white border rounded-xl p-3">
          <div className="grid grid-cols-7 text-center text-[11px] text-gray-500 mb-1">
            {["일", "월", "화", "수", "목", "금", "토"].map((d) => <div key={d} className="py-1">{d}</div>)}
          </div>
          <div className="grid grid-cols-7 gap-1">
            {Array.from({ length: skip }).map((_, i) => <div key={`pad-${i}`} />)}
            {Array.from({ length: days }, (_, i) => i + 1).map((d) => {
              const ds = `${ym}-${String(d).padStart(2, "0")}`;
              const w = wishMap.get(ds);
              const dow = new Date(ds + "T00:00:00").getDay();
              const weekend = dow === 0 || dow === 6;
              return (
                <button
                  key={d}
                  onClick={() => !isHead && setPicker({ date: ds, current: w })}
                  disabled={isHead}
                  className={clsx(
                    "rounded-md border min-h-[64px] p-1.5 text-left transition",
                    weekend ? "bg-rose-50" : "bg-white",
                    !isHead && "hover:border-blue-500 cursor-pointer",
                    isHead && "cursor-default",
                  )}
                >
                  <div className="text-[11px] font-medium text-gray-700">{d}</div>
                  {w && (
                    <div className={clsx("mt-1 text-[11px] rounded px-1.5 py-0.5 inline-flex items-center gap-1", TYPE_BG[w.type])}>
                      <span>{TYPE_EMOJI[w.type]}</span>
                      <span>{TYPE_LABEL[w.type]}</span>
                    </div>
                  )}
                </button>
              );
            })}
          </div>
        </div>
      )}

      {picker && (
        <WishPicker
          date={picker.date}
          current={picker.current}
          onClose={() => setPicker(null)}
          onPick={(type, reason) => {
            upsert.mutate({ date: picker.date, body: { type, reason } });
            setPicker(null);
          }}
          onDelete={() => {
            del.mutate(picker.date);
            setPicker(null);
          }}
        />
      )}
    </div>
  );
}

function WishPicker({ date, current, onClose, onPick, onDelete }: {
  date: string; current?: Wish; onClose: () => void;
  onPick: (t: WishType, reason?: string) => void; onDelete: () => void;
}) {
  const [reason, setReason] = useState(current?.reason ?? "");
  const types: WishType[] = ["off", "d", "e", "n", "unavailable"];
  return (
    <Modal open onClose={onClose} title={`${date} 희망일`} size="sm">
      <div className="grid grid-cols-5 gap-2 mb-3">
        {types.map((t) => (
          <button
            key={t}
            onClick={() => onPick(t, reason || undefined)}
            className={clsx("rounded-md py-3 transition", TYPE_BG[t], current?.type === t && "ring-2 ring-blue-500", "hover:opacity-90")}
          >
            <div className="text-lg">{TYPE_EMOJI[t]}</div>
            <div className="text-xs font-medium mt-1">{TYPE_LABEL[t]}</div>
          </button>
        ))}
      </div>
      <label className="block">
        <span className="block text-xs text-gray-600 mb-1">사유 (unavailable 권장)</span>
        <input className="input" value={reason} onChange={(e) => setReason(e.target.value)} />
      </label>
      <div className="flex justify-between mt-4">
        {current ? (
          <Button variant="danger" size="sm" onClick={onDelete}>삭제</Button>
        ) : <span />}
        <Button variant="ghost" onClick={onClose}>닫기</Button>
      </div>
    </Modal>
  );
}
