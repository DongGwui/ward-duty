"use client";
import Link from "next/link";

function thisMonth() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

export default function HomePage() {
  const ym = thisMonth();
  const next = new Date();
  next.setMonth(next.getMonth() + 1);
  const nextYm = `${next.getFullYear()}-${String(next.getMonth() + 1).padStart(2, "0")}`;

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">대시보드</h1>
      <div className="grid sm:grid-cols-2 gap-4">
        <Link
          href={`/duty/${ym}`}
          className="block bg-white rounded-xl border p-5 hover:border-blue-500 transition"
        >
          <div className="text-sm text-gray-500 mb-1">이번 달</div>
          <div className="text-xl font-semibold">{ym} 듀티</div>
        </Link>
        <Link
          href={`/duty/${nextYm}`}
          className="block bg-white rounded-xl border p-5 hover:border-blue-500 transition"
        >
          <div className="text-sm text-gray-500 mb-1">다음 달</div>
          <div className="text-xl font-semibold">{nextYm} 듀티</div>
        </Link>
      </div>
    </div>
  );
}
