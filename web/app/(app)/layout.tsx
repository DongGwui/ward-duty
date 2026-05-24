"use client";
import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import type { Subject } from "@/lib/types";
import { Nav } from "@/components/layout/Nav";
import { Spinner } from "@/components/ui/Spinner";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const { data: me, isLoading, isError } = useQuery<Subject>({
    queryKey: ["me"],
    queryFn: () => apiFetch<Subject>("/auth/me"),
    retry: false,
  });

  // 로딩이 끝났는데 me가 없으면(=401/네트워크 에러 등) /login으로 강제 이동.
  // error 타입 의존을 없애 어떤 실패에도 redirect 보장.
  useEffect(() => {
    if (!isLoading && (isError || !me)) {
      router.replace("/login");
    }
  }, [isLoading, isError, me, router]);

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <Spinner size={6} />
      </div>
    );
  }
  if (!me) {
    // redirect 대기 중 — 흰 화면 대신 명시적 안내
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-sm text-gray-500">
          로그인이 필요합니다.{" "}
          <a className="text-blue-600 underline" href="/login">
            /login
          </a>{" "}
          으로 이동 중…
        </div>
      </div>
    );
  }

  return (
    <>
      <Nav />
      <main className="max-w-7xl mx-auto px-4 py-6">{children}</main>
    </>
  );
}
