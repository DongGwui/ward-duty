"use client";
import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { apiFetch, ApiException } from "@/lib/api";
import type { Subject } from "@/lib/types";
import { Nav } from "@/components/layout/Nav";
import { Spinner } from "@/components/ui/Spinner";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const { data: me, isLoading, error } = useQuery<Subject>({
    queryKey: ["me"],
    queryFn: () => apiFetch<Subject>("/auth/me"),
    retry: false,
  });

  useEffect(() => {
    if (error instanceof ApiException && error.status === 401) {
      router.replace("/login");
    }
  }, [error, router]);

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <Spinner size={6} />
      </div>
    );
  }
  if (!me) return null;

  return (
    <>
      <Nav />
      <main className="max-w-7xl mx-auto px-4 py-6">{children}</main>
    </>
  );
}
