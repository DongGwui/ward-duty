import type { Config } from "tailwindcss";

const config: Config = {
  content: ["./app/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}", "./features/**/*.{ts,tsx}"],
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
