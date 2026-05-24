"use client";
import { useState, useEffect } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import { Button } from "@/components/ui/Button";
import { Spinner } from "@/components/ui/Spinner";
import type { Subject } from "@/lib/types";

interface Settings {
  min_d: number; min_e: number; min_n: number;
  max_consecutive_n: number; min_rest_after_n: number; max_consecutive_workdays: number;
  balance_off_tolerance: number;
  previous_month_lookback_days: number;
  night_keeper_max_consecutive_months: number;
  night_keeper_cooldown_months: number;
  wish_unavailable_quota_monthly: number | null;
  wish_preference_quota_monthly: number | null;
  wish_deadline_days_before_month: number;
  swap_deadline_days_before_date: number;
  weight_balance_off: number;
  weight_respect_wishes: number;
  weight_weekend_balance: number;
  weight_same_shift_streak: number;
  weight_short_rest_pattern: number;
}

const GROUPS: { title: string; fields: (keyof Settings)[]; desc?: string }[] = [
  { title: "시프트별 최소 인원 (H-04)", fields: ["min_d", "min_e", "min_n"] },
  { title: "Hard 제약 임계값", fields: ["max_consecutive_n", "min_rest_after_n", "max_consecutive_workdays", "balance_off_tolerance"], desc: "H-02 · H-06 · H-03 · S-01" },
  { title: "월 경계 / 나이트킵 (H-10 · K-NN)", fields: ["previous_month_lookback_days", "night_keeper_max_consecutive_months", "night_keeper_cooldown_months"] },
  { title: "희망일 · Swap 운영 (W · X)", fields: ["wish_unavailable_quota_monthly", "wish_preference_quota_monthly", "wish_deadline_days_before_month", "swap_deadline_days_before_date"] },
  { title: "Soft 가중치 (S)", fields: ["weight_balance_off", "weight_respect_wishes", "weight_weekend_balance", "weight_same_shift_streak", "weight_short_rest_pattern"], desc: "값이 클수록 우선" },
];

const LABELS: Record<keyof Settings, string> = {
  min_d: "min_d (Day)", min_e: "min_e (Evening)", min_n: "min_n (Night)",
  max_consecutive_n: "max_consecutive_n", min_rest_after_n: "min_rest_after_n",
  max_consecutive_workdays: "max_consecutive_workdays", balance_off_tolerance: "balance_off_tolerance",
  previous_month_lookback_days: "previous_month_lookback_days",
  night_keeper_max_consecutive_months: "nk_max_consecutive_months",
  night_keeper_cooldown_months: "nk_cooldown_months",
  wish_unavailable_quota_monthly: "wish_unavailable_quota_monthly (null=무제한)",
  wish_preference_quota_monthly: "wish_preference_quota_monthly (null=무제한)",
  wish_deadline_days_before_month: "wish_deadline_days_before_month",
  swap_deadline_days_before_date: "swap_deadline_days_before_date",
  weight_balance_off: "weight_balance_off (S-01)",
  weight_respect_wishes: "weight_respect_wishes (S-02)",
  weight_weekend_balance: "weight_weekend_balance (S-03)",
  weight_same_shift_streak: "weight_same_shift_streak (S-04)",
  weight_short_rest_pattern: "weight_short_rest_pattern (S-05)",
};

export default function SettingsPage() {
  const { data: me } = useQuery<Subject>({ queryKey: ["me"], queryFn: () => apiFetch<Subject>("/auth/me") });
  const isHead = me?.rl === "head_nurse";
  const q = useQuery<Settings>({ queryKey: ["settings"], queryFn: () => apiFetch<Settings>("/settings") });
  const [form, setForm] = useState<Partial<Settings>>({});

  useEffect(() => {
    if (q.data) setForm(q.data);
  }, [q.data]);

  const save = useMutation({
    mutationFn: (patch: Partial<Settings>) => apiFetch<Settings>("/settings", { method: "PATCH", body: patch }),
    onSuccess: (s) => setForm(s),
  });

  if (q.isLoading) return <Spinner size={6} />;
  if (!q.data) return <p>설정을 불러올 수 없습니다.</p>;

  function setNum(k: keyof Settings, v: string) {
    const n = v === "" ? null : Number(v);
    setForm((f) => ({ ...f, [k]: n }));
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">설정 (ward_settings)</h1>
        {isHead && (
          <Button onClick={() => save.mutate(form)} disabled={save.isPending}>
            {save.isPending ? "저장 중…" : "저장"}
          </Button>
        )}
      </div>
      {!isHead && <p className="text-sm text-yellow-700 bg-yellow-50 border border-yellow-200 rounded px-3 py-2">조회 전용 — 수간호사만 수정 가능</p>}

      {GROUPS.map((g) => (
        <section key={g.title} className="bg-white border rounded-xl p-5">
          <h2 className="font-semibold mb-1">{g.title}</h2>
          {g.desc && <p className="text-xs text-gray-500 mb-3">{g.desc}</p>}
          <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {g.fields.map((f) => (
              <label key={f} className="block">
                <span className="block text-xs text-gray-600 mb-1 font-mono">{LABELS[f]}</span>
                <input
                  type="number"
                  className="w-full rounded-md border px-2 py-1.5 text-sm disabled:bg-gray-50"
                  disabled={!isHead}
                  value={form[f] === null || form[f] === undefined ? "" : String(form[f])}
                  onChange={(e) => setNum(f, e.target.value)}
                />
              </label>
            ))}
          </div>
        </section>
      ))}

      {save.error && (
        <div className="bg-red-50 border border-red-200 rounded px-3 py-2 text-sm text-red-800">
          저장 실패: {(save.error as Error).message}
        </div>
      )}
    </div>
  );
}
