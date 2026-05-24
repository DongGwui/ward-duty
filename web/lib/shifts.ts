// 시프트 색상·라벨·이모지.

import type { ShiftCode } from "./types";

export const SHIFT_LABEL: Record<ShiftCode, string> = {
  D: "Day",
  E: "Evening",
  N: "Night",
  O: "Off",
  DE: "Double",
};

export const SHIFT_BG: Record<ShiftCode, string> = {
  D: "bg-shift-D",
  E: "bg-shift-E",
  N: "bg-shift-N",
  O: "bg-shift-O",
  DE: "bg-shift-DE",
};

export const SHIFT_TEXT: Record<ShiftCode, string> = {
  D: "text-white",
  E: "text-white",
  N: "text-white",
  O: "text-gray-700",
  DE: "text-white",
};

export const SHIFT_EMOJI: Record<ShiftCode, string> = {
  D: "☀️",
  E: "🌆",
  N: "🌙",
  O: "🛌",
  DE: "🔥",
};

export const ALL_SHIFTS: ShiftCode[] = ["D", "E", "N", "O", "DE"];
export const AUTO_SHIFTS: ShiftCode[] = ["D", "E", "N", "O"]; // H-13
