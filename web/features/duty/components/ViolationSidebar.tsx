"use client";
import type { Violation } from "@/lib/types";
import { Badge } from "@/components/ui/Badge";
import { getRuleMessage } from "@/lib/violations";

interface Props {
  violations: Violation[];
  hardCount?: number;
  softCount?: number;
}

export function ViolationSidebar({ violations, hardCount, softCount }: Props) {
  if (!violations.length) {
    return (
      <div className="bg-green-50 border border-green-200 rounded-lg p-4">
        <div className="font-semibold text-green-800">규칙 위반 없음 ✓</div>
        <div className="text-xs text-green-700 mt-1">확정 가능한 상태입니다.</div>
      </div>
    );
  }
  return (
    <div className="bg-white border rounded-lg p-4 space-y-3 max-h-[480px] overflow-y-auto">
      <div className="flex items-center justify-between">
        <div className="font-semibold">규칙 위반</div>
        <div className="flex gap-2">
          {hardCount! > 0 && <Badge variant="danger">hard {hardCount}</Badge>}
          {softCount! > 0 && <Badge variant="warning">soft {softCount}</Badge>}
        </div>
      </div>
      <ul className="space-y-2 text-sm">
        {violations.map((v, i) => {
          const m = getRuleMessage(v.rule_id);
          return (
            <li
              key={i}
              className={
                v.severity === "hard"
                  ? "border-l-4 border-red-500 pl-3 py-1"
                  : "border-l-4 border-yellow-400 pl-3 py-1"
              }
            >
              <div className="font-mono text-[11px] text-gray-500">{v.rule_id}</div>
              <div className="font-medium">{m.title}</div>
              <div className="text-xs text-gray-600 mt-0.5">
                {v.nurse_id && `nurse #${v.nurse_id}`}
                {v.date && ` · ${v.date}`}
              </div>
              {v.message && <div className="text-xs text-gray-500 mt-0.5">{v.message}</div>}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
