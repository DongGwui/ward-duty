import { loginUrl } from "@/lib/api";

export default function LoginPage() {
  return (
    <main className="min-h-screen flex items-center justify-center px-4">
      <div className="max-w-sm w-full bg-white rounded-xl shadow p-8 text-center">
        <h1 className="text-2xl font-bold mb-2">ward-duty</h1>
        <p className="text-sm text-gray-600 mb-6">
          간호사 듀티 자동 생성 도구.
          <br />
          수간호사가 등록한 이메일로만 로그인할 수 있습니다.
        </p>
        <a
          href={loginUrl()}
          className="inline-flex items-center justify-center gap-2 w-full px-4 py-2.5 bg-white border border-gray-300 rounded-md hover:bg-gray-50 transition"
        >
          <svg width="18" height="18" viewBox="0 0 18 18" aria-hidden="true">
            <path
              fill="#4285F4"
              d="M17.64 9.2c0-.637-.057-1.251-.164-1.84H9v3.481h4.844a4.14 4.14 0 0 1-1.796 2.716v2.259h2.908c1.702-1.567 2.684-3.875 2.684-6.615z"
            />
            <path
              fill="#34A853"
              d="M9 18c2.43 0 4.467-.806 5.956-2.18l-2.908-2.259c-.806.54-1.837.86-3.048.86-2.344 0-4.328-1.584-5.036-3.711H.957v2.332A8.997 8.997 0 0 0 9 18z"
            />
            <path
              fill="#FBBC05"
              d="M3.964 10.71A5.41 5.41 0 0 1 3.682 9c0-.593.102-1.17.282-1.71V4.958H.957A8.996 8.996 0 0 0 0 9c0 1.452.348 2.827.957 4.042l3.007-2.332z"
            />
            <path
              fill="#EA4335"
              d="M9 3.58c1.321 0 2.508.454 3.44 1.345l2.582-2.58C13.463.891 11.426 0 9 0A8.997 8.997 0 0 0 .957 4.958L3.964 7.29C4.672 5.163 6.656 3.58 9 3.58z"
            />
          </svg>
          <span className="text-sm font-medium">Google로 로그인</span>
        </a>
      </div>
    </main>
  );
}
