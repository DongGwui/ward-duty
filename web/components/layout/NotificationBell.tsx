"use client";
import { useState, useRef, useEffect } from "react";
import { useRouter } from "next/navigation";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch, apiFetchFull } from "@/lib/api";
import type { Notification } from "@/lib/types";
import clsx from "clsx";

const TYPE_ICON: Record<string, string> = {
  account_pending_approval: "🔔",
  swap_request_received: "📥",
  swap_b_accepted: "🤝",
  swap_approved: "✅",
  swap_rejected: "❌",
  level_changed: "🏷️",
  fixed_pattern_changed: "🔧",
  nightkeeper_assigned: "🌙",
  schedule_confirmed: "📅",
};

export function NotificationBell() {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const router = useRouter();
  const qc = useQueryClient();

  const count = useQuery({
    queryKey: ["notifications", "unread-count"],
    queryFn: () => apiFetch<{ unread_count: number }>("/notifications/unread-count"),
    refetchInterval: 15_000,
  });

  const list = useQuery({
    queryKey: ["notifications", "recent"],
    queryFn: () =>
      apiFetchFull<Notification[]>("/notifications", { search: { limit: 10 } }),
    enabled: open,
  });

  const markRead = useMutation({
    mutationFn: (id: number) => apiFetch(`/notifications/${id}/read`, { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });
  const readAll = useMutation({
    mutationFn: () => apiFetch("/notifications/read-all", { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });

  useEffect(() => {
    if (!open) return;
    const h = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", h);
    return () => document.removeEventListener("mousedown", h);
  }, [open]);

  const unread = count.data?.unread_count ?? 0;

  function onClickItem(n: Notification) {
    if (!n.read_at) markRead.mutate(n.id);
    if (n.link) router.push(n.link);
    setOpen(false);
  }

  return (
    <div className="relative" ref={ref}>
      <button
        onClick={() => setOpen((v) => !v)}
        className="relative w-9 h-9 rounded-full hover:bg-gray-100 flex items-center justify-center transition"
        aria-label="알림"
      >
        <span className="text-lg">🔔</span>
        {unread > 0 && (
          <span className="absolute top-0.5 right-0.5 bg-red-500 text-white text-[10px] font-bold rounded-full min-w-[16px] h-4 px-1 flex items-center justify-center">
            {unread > 9 ? "9+" : unread}
          </span>
        )}
      </button>

      {open && (
        <div className="absolute right-0 mt-2 w-80 max-w-[90vw] bg-white border rounded-lg shadow-lg z-50">
          <div className="flex items-center justify-between px-3 py-2 border-b">
            <div className="font-semibold text-sm">
              알림{" "}
              {unread > 0 && <span className="text-red-500">({unread})</span>}
            </div>
            <div className="flex gap-2">
              {unread > 0 && (
                <button className="text-xs text-blue-600 hover:underline" onClick={() => readAll.mutate()}>
                  모두 읽음
                </button>
              )}
              <button
                className="text-xs text-gray-600 hover:underline"
                onClick={() => {
                  setOpen(false);
                  router.push("/notifications");
                }}
              >
                모두 보기
              </button>
            </div>
          </div>
          <div className="max-h-96 overflow-y-auto">
            {list.isLoading && <div className="p-4 text-center text-sm text-gray-400">불러오는 중…</div>}
            {!list.isLoading && (list.data?.data?.length ?? 0) === 0 && (
              <div className="p-6 text-center text-sm text-gray-400">알림이 없습니다.</div>
            )}
            {(list.data?.data ?? []).map((n) => (
              <button
                key={n.id}
                onClick={() => onClickItem(n)}
                className={clsx(
                  "w-full text-left px-3 py-2.5 border-b last:border-b-0 hover:bg-gray-50 flex gap-2.5 transition",
                  !n.read_at && "bg-blue-50/40",
                )}
              >
                <span className="text-base mt-0.5">{TYPE_ICON[n.type] ?? "📌"}</span>
                <div className="flex-1 min-w-0">
                  <div className="flex items-start justify-between gap-2">
                    <div className="font-medium text-sm truncate">{n.title}</div>
                    {!n.read_at && <span className="w-2 h-2 bg-red-500 rounded-full mt-1.5 shrink-0" />}
                  </div>
                  {n.body && <div className="text-xs text-gray-600 truncate mt-0.5">{n.body}</div>}
                  <div className="text-[10px] text-gray-400 mt-1">{relativeTime(n.created_at)}</div>
                </div>
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function relativeTime(iso: string): string {
  const t = new Date(iso).getTime();
  const diff = Date.now() - t;
  const min = Math.floor(diff / 60_000);
  if (min < 1) return "방금";
  if (min < 60) return `${min}분 전`;
  const h = Math.floor(min / 60);
  if (h < 24) return `${h}시간 전`;
  const d = Math.floor(h / 24);
  if (d < 7) return `${d}일 전`;
  return new Date(iso).toLocaleDateString();
}
