import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./app/**/*.{ts,tsx}",
    "./components/**/*.{ts,tsx}",
    "./features/**/*.{ts,tsx}",
    "./lib/**/*.{ts,tsx}", // lib/shifts.ts 의 SHIFT_BG/SHIFT_TEXT 매핑 문자열 추출
  ],
  // 동적 조합(`bg-shift-${code}`)이 아닌 명시적 매핑이지만, custom color는
  // content 추출 누락 시 통째로 빠지므로 보험으로 safelist 명시.
  safelist: [
    "bg-shift-D", "bg-shift-E", "bg-shift-N", "bg-shift-O", "bg-shift-DE",
    "text-shift-D", "text-shift-E", "text-shift-N", "text-shift-O", "text-shift-DE",
  ],
  theme: {
    extend: {
      colors: {
        // 시프트 색상 (Design §5.4)
        shift: {
          D: "#2563eb",  // 파랑
          E: "#ea580c",  // 주황
          N: "#7c3aed",  // 보라
          O: "#9ca3af",  // 회색
          DE: "#dc2626", // 빨강 (DE는 부득이한 더블)
        },
      },
    },
  },
  plugins: [],
};
export default config;
