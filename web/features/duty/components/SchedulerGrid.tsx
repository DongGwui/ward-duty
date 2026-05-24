"use client";
import { useMemo, useState } from "react";
import clsx from "clsx";
import type { Nurse, ScheduleCell, ShiftCode, Violation } from "@/lib/types";
import { SHIFT_BG, SHIFT_TEXT } from "@/lib/shifts";
import { ShiftPicker } from "./ShiftPicker";
import { getRuleMessage } from "@/lib/violations";

interface Props {
  yearMonth: string;
  nurses: Nurse[];
  cells: ScheduleCell[];
  violations: Violation[];
  canEdit: boolean;
  onPatch: (cellId: number, shift: ShiftCode) => void;
}

function daysInMonth(ym: string): number {
  const [y, m] = ym.split("-").map(Number);
  return new Date(y, m, 0).getDate();
}

function dateStr(ym: string, day: number): string {
  return `${ym}-${String(day).padStart(2, "0")}`;
}

export function SchedulerGrid({ yearMonth, nurses, cells, violations, canEdit, onPatch }: Props) {
  const [picker, setPicker] = useState<{ cell: ScheduleCell; nurse: Nurse } | null>(null);
  const days = daysInMonth(yearMonth);

  // (nurse_id, date) → cell
  const byKey = useMemo(() => {
    const m = new Map<string, ScheduleCell>();
    for (const c of cells) m.set(`${c.nurse_id}|${c.date}`, c);
    return m;
  }, [cells]);

  // cell_id → violations (severity 우선순위: hard > soft)
  const cellViolations = useMemo(() => {
    const m = new Map<number, Violation[]>();
    for (const v of violations) {
      for (const id of v.cell_ids ?? []) {
        const arr = m.get(id) ?? [];
        arr.push(v);
        m.set(id, arr);
      }
    }
    return m;
  }, [violations]);

  return (
    <>
      <div className="overflow-x-auto rounded-lg border bg-white">
        <table className="text-xs border-collapse min-w-full">
          <thead>
            <tr className="bg-gray-50 sticky top-0">
              <th className="sticky left-0 z-10 bg-gray-50 border-b px-2 py-2 text-left w-32">
                간호사
              </th>
              {Array.from({ length: days }, (_, i) => i + 1).map((d) => {
                const dow = new Date(`${dateStr(yearMonth, d)}T00:00:00`).getDay();
                const isWeekend = dow === 0 || dow === 6;
                return (
                  <th
                    key={d}
                    className={clsx(
                      "border-b px-1 py-1 text-center w-9 font-medium",
                      isWeekend && "bg-rose-50 text-rose-700",
                    )}
                  >
                    {d}
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody>
            {nurses.map((n) => (
              <tr key={n.id} className="hover:bg-gray-50/60">
                <td className="sticky left-0 z-10 bg-white border-b px-2 py-1">
                  <div className="font-medium truncate">{n.name}</div>
                  <div className="text-[10px] text-gray-500">
                    {n.resolved_level ?? ""}
                    {n.fixed_shift_pattern ? ` · ${n.fixed_shift_pattern}` : ""}
                  </div>
                </td>
                {Array.from({ length: days }, (_, i) => i + 1).map((d) => {
                  const ds = dateStr(yearMonth, d);
                  const cell = byKey.get(`${n.id}|${ds}`);
                  if (!cell) {
                    return (
                      <td key={d} className="border-b text-center text-gray-300">
                        -
                      </td>
                    );
                  }
                  const vs = cellViolations.get(cell.id) ?? [];
                  const hasHard = vs.some((v) => v.severity === "hard");
                  const hasSoft = vs.some((v) => v.severity === "soft");
                  return (
                    <td
                      key={d}
                      className={clsx(
                        "border-b p-0 text-center",
                        hasHard && "ring-2 ring-red-500 ring-inset",
                        !hasHard && hasSoft && "ring-2 ring-yellow-400 ring-inset",
                      )}
                      title={
                        vs.length
                          ? vs.map((v) => `[${v.rule_id}] ${getRuleMessage(v.rule_id).title}`).join("\n")
                          : undefined
                      }
                    >
                      <button
                        disabled={!canEdit}
                        onClick={() => canEdit && setPicker({ cell, nurse: n })}
                        className={clsx(
                          "w-full h-7 font-semibold",
                          SHIFT_BG[cell.shift],
                          SHIFT_TEXT[cell.shift],
                          cell.source === "manual" && "underline decoration-2 decoration-white/60",
                          !canEdit && "cursor-default",
                        )}
                      >
                        {cell.shift === "DE" ? "DE" : cell.shift}
                      </button>
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {picker && (
        <ShiftPicker
          open
          current={picker.cell.shift}
          onClose={() => setPicker(null)}
          context={`${picker.nurse.name} · ${picker.cell.date}`}
          onPick={(s) => onPatch(picker.cell.id, s)}
        />
      )}
    </>
  );
}
