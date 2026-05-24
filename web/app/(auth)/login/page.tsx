"use client";
import { useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { apiFetch, loginUrl } from "@/lib/api";
import { Button } from "@/components/ui/Button";

export default function LoginPage() {
  const sp = useSearchParams();
  const pending = sp.get("pending") === "1";
  const pendingEmail = sp.get("email") ?? "";

  // 로컬 개발 모드 — .env.local에 NEXT_PUBLIC_DEV_LOGIN=1
  const devMode = process.env.NEXT_PUBLIC_DEV_LOGIN === "1";
  const router = useRouter();
  const qc = useQueryClient();
  const [email, setEmail] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function devLogin() {
    setError(null);
    setLoading(true);
    try {
      await apiFetch("/auth/dev-login", { method: "POST", body: { email } });
      await qc.invalidateQueries({ queryKey: ["me"] });
      router.replace("/");
    } catch (e: unknown) {
      const err = e as { status?: number; body?: { code?: string; message?: string } };
      if (err.body?.code === "PENDING_APPROVAL") {
        // Stage 2: 매니저 매칭 대기
        router.replace(`/login?pending=1&email=${encodeURIComponent(email)}`);
        return;
      }
      setError(err.body?.message ?? "로그인 실패");
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="min-h-screen flex items-center justify-center px-4">
      <div className="max-w-sm w-full bg-white rounded-xl shadow p-8">
        <h1 className="text-2xl font-bold mb-2 text-center">ward-duty</h1>
        <p className="text-sm text-gray-600 mb-6 text-center">
          간호사 듀티 자동 생성 도구.
          <br />
          매니저가 등록한 이메일로만 로그인할 수 있습니다.
        </p>

        {pending && (
          <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-3 mb-4 text-sm">
            <div className="font-semibold text-yellow-900">매니저 승인 대기</div>
            <p className="text-yellow-800 mt-1 text-xs">
              {pendingEmail && <span className="font-mono">{pendingEmail}</span>}
              {pendingEmail && " "}계정이 명단에 연결될 때까지 잠시 기다려주세요. 매니저가
              명단에서 본인을 연결해 주면 다시 로그인할 수 있습니다.
            </p>
          </div>
        )}

        <a
          href={loginUrl()}
          className="inline-flex items-center justify-center gap-2 w-full px-4 py-2.5 bg-white border border-gray-300 rounded-md hover:bg-gray-50 transition"
        >
          <svg width="18" height="18" viewBox="0 0 18 18" aria-hidden="true">
            <path fill="#4285F4" d="M17.64 9.2c0-.637-.057-1.251-.164-1.84H9v3.481h4.844a4.14 4.14 0 0 1-1.796 2.716v2.259h2.908c1.702-1.567 2.684-3.875 2.684-6.615z" />
            <path fill="#34A853" d="M9 18c2.43 0 4.467-.806 5.956-2.18l-2.908-2.259c-.806.54-1.837.86-3.048.86-2.344 0-4.328-1.584-5.036-3.711H.957v2.332A8.997 8.997 0 0 0 9 18z" />
            <path fill="#FBBC05" d="M3.964 10.71A5.41 5.41 0 0 1 3.682 9c0-.593.102-1.17.282-1.71V4.958H.957A8.996 8.996 0 0 0 0 9c0 1.452.348 2.827.957 4.042l3.007-2.332z" />
            <path fill="#EA4335" d="M9 3.58c1.321 0 2.508.454 3.44 1.345l2.582-2.58C13.463.891 11.426 0 9 0A8.997 8.997 0 0 0 .957 4.958L3.964 7.29C4.672 5.163 6.656 3.58 9 3.58z" />
          </svg>
          <span className="text-sm font-medium">Google로 로그인</span>
        </a>

        {devMode && (
          <div className="mt-6 pt-6 border-t border-dashed border-yellow-400">
            <div className="text-xs text-yellow-700 mb-2 font-mono">⚙️ DEV LOGIN (ENV=development)</div>
            <input
              type="email"
              className="input mb-2"
              placeholder="이메일 입력"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
            {error && <p className="text-xs text-red-600 mb-2">{error}</p>}
            <Button onClick={devLogin} disabled={!email || loading} className="w-full">
              {loading ? "로그인 중…" : "Dev Login"}
            </Button>
            <p className="text-[10px] text-gray-500 mt-2">
              ADMIN_EMAIL과 일치하면 head_nurse 자동 시드. 그 외는 nurses에 등록된 이메일만 가능.
            </p>
          </div>
        )}
      </div>
    </main>
  );
}
