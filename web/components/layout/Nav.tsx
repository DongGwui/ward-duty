"use client";
import Link from "next/link";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import type { Subject } from "@/lib/types";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";

function thisMonth() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

export function Nav() {
  const qc = useQueryClient();
  const { data: me } = useQuery<Subject>({ queryKey: ["me"], queryFn: () => apiFetch<Subject>("/auth/me") });
  const logout = useMutation({
    mutationFn: () => apiFetch("/auth/logout", { method: "POST" }),
    onSuccess: () => {
      qc.clear();
      window.location.href = "/login";
    },
  });

  const ym = thisMonth();

  return (
    <nav className="bg-white border-b">
      <div className="max-w-7xl mx-auto px-4 py-3 flex items-center justify-between">
        <div className="flex items-center gap-6">
          <Link href="/" className="font-bold text-lg">
            ward-duty
          </Link>
          <div className="flex items-center gap-4 text-sm">
            <Link href={`/duty/${ym}`} className="hover:text-blue-600">
              듀티
            </Link>
            {me?.rl === "head_nurse" && (
              <>
                <Link href="/nurses" className="hover:text-blue-600">명단</Link>
                <Link href="/levels" className="hover:text-blue-600">등급</Link>
                <Link href="/settings" className="hover:text-blue-600">설정</Link>
                <Link href="/night-keepers" className="hover:text-blue-600">나이트킵</Link>
              </>
            )}
            <Link href="/wishes" className="hover:text-blue-600">희망일</Link>
            <Link href="/swaps" className="hover:text-blue-600">교환</Link>
          </div>
        </div>
        <div className="flex items-center gap-3">
          {me && (
            <>
              <span className="text-sm text-gray-600">{me.em}</span>
              {me.rl === "head_nurse" && <Badge variant="info">매니저</Badge>}
              <Button variant="ghost" size="sm" onClick={() => logout.mutate()}>
                로그아웃
              </Button>
            </>
          )}
        </div>
      </div>
    </nav>
  );
}
