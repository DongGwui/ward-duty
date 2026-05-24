import type { Metadata } from "next";
import "./globals.css";
import { Providers } from "@/lib/queryClient";

export const metadata: Metadata = {
  title: "ward-duty",
  description: "간호사 듀티 자동 생성",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="ko">
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
