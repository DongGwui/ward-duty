"use client";
import type { Schedule } from "@/lib/types";
import { Spinner } from "@/components/ui/Spinner";
import { Badge } from "@/components/ui/Badge";

interface Props {
  schedule?: Schedule;
}

export function StatusBar({ schedule }: Props) {
  if (!schedule) return null;
  const s = schedule.status;
  if (s === "generating") {
    return (
      <div className="flex items-center gap-3 bg-blue-50 border border-blue-200 rounded-lg px-4 py-3">
        <Spinner size={5} />
        <div>
          <div className="font-medium text-blue-900">자동 생성 중…</div>
          <div className="text-xs text-blue-700">30명 × 31일 기준 보통 1분 이내</div>
        </div>
      </div>
    );
  }
  if (s === "failed") {
    const log = schedule.generation_log;
    return (
      <div className="bg-red-50 border border-red-200 rounded-lg px-4 py-3">
        <div className="font-medium text-red-900">
          생성 실패
          {log?.solver_status ? ` (${log.solver_status})` : ""}
        </div>
        {log?.suggestion && <div className="text-sm text-red-800 mt-1">{log.suggestion}</div>}
        {log?.violated_rule_ids?.length ? (
          <div className="text-xs text-red-700 mt-2">
            추정 위반 룰: {log.violated_rule_ids.join(", ")}
          </div>
        ) : null}
      </div>
    );
  }
  if (s === "confirmed") {
    return (
      <div className="bg-green-50 border border-green-200 rounded-lg px-4 py-3">
        <div className="font-medium text-green-900">확정됨 ✓</div>
        {schedule.confirmed_at && (
          <div className="text-xs text-green-800">{new Date(schedule.confirmed_at).toLocaleString()}</div>
        )}
      </div>
    );
  }
  if (s === "generated") {
    return (
      <div className="bg-white border rounded-lg px-4 py-3">
        <Badge variant="info">생성 완료 — 검토 후 확정</Badge>
      </div>
    );
  }
  return (
    <div className="bg-gray-50 border rounded-lg px-4 py-3">
      <Badge>초안 (Generate 클릭)</Badge>
    </div>
  );
}
