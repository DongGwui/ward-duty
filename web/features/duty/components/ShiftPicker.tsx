"use client";
import { Modal } from "@/components/ui/Modal";
import { ALL_SHIFTS, SHIFT_BG, SHIFT_TEXT, SHIFT_LABEL, SHIFT_EMOJI } from "@/lib/shifts";
import type { ShiftCode } from "@/lib/types";
import clsx from "clsx";

interface Props {
  open: boolean;
  current?: ShiftCode;
  onPick: (s: ShiftCode) => void;
  onClose: () => void;
  /** 해당 nurse·date의 컨텍스트 정보 (선택, 표시용) */
  context?: string;
}

export function ShiftPicker({ open, current, onPick, onClose, context }: Props) {
  return (
    <Modal open={open} onClose={onClose} title="시프트 선택" size="sm">
      {context && <p className="text-xs text-gray-500 mb-3">{context}</p>}
      <div className="grid grid-cols-3 gap-2">
        {ALL_SHIFTS.map((s) => (
          <button
            key={s}
            onClick={() => {
              onPick(s);
              onClose();
            }}
            className={clsx(
              "rounded-md px-3 py-3 text-center transition",
              SHIFT_BG[s],
              SHIFT_TEXT[s],
              current === s ? "ring-2 ring-offset-2 ring-blue-500" : "hover:opacity-90",
            )}
          >
            <div className="text-lg">{SHIFT_EMOJI[s]}</div>
            <div className="text-sm font-semibold mt-1">{s}</div>
            <div className="text-[10px] opacity-80">{SHIFT_LABEL[s]}</div>
          </button>
        ))}
      </div>
      {/* H-13 안내: DE는 부득이한 더블 */}
      <p className="text-[11px] text-gray-500 mt-3">
        DE(더블)는 부득이한 인력 부족 시에만. 자동 생성은 DE를 만들지 않습니다.
      </p>
    </Modal>
  );
}
