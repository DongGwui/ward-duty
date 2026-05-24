"use client";
import { useRouter } from "next/navigation";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch, apiFetchFull } from "@/lib/api";
import { Button } from "@/components/ui/Button";
import { Spinner } from "@/components/ui/Spinner";
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

export default function NotificationsPage() {
  const router = useRouter();
  const qc = useQueryClient();

  const list = useQuery({
    queryKey: ["notifications", "all"],
    queryFn: () =>
      apiFetchFull<Notification[]>("/notifications", { search: { limit: 100 } }),
  });

  const markRead = useMutation({
    mutationFn: (id: number) => apiFetch(`/notifications/${id}/read`, { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });
  const readAll = useMutation({
    mutationFn: () => apiFetch("/notifications/read-all", { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });
  const del = useMutation({
    mutationFn: (id: number) => apiFetch(`/notifications/${id}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });

  if (list.isLoading) return <Spinner size={6} />;

  const items = list.data?.data ?? [];
  const unreadMeta = (list.data?.meta as { unread_count?: number } | undefined)?.unread_count ?? 0;

  function onClickItem(n: Notification) {
    if (!n.read_at) markRead.mutate(n.id);
    if (n.link) router.push(n.link);
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">알림</h1>
          <p className="text-sm text-gray-600 mt-1">최근 100건의 알림을 표시합니다.</p>
        </div>
        {unreadMeta > 0 && (
          <Button variant="secondary" size="sm" onClick={() => readAll.mutate()}>
            모두 읽음 ({unreadMeta})
          </Button>
        )}
      </div>

      {items.length === 0 ? (
        <div className="bg-white border rounded-xl py-16 text-center">
          <div className="text-4xl opacity-30 mb-2">🔕</div>
          <p className="text-sm text-gray-500">알림이 없습니다.</p>
        </div>
      ) : (
        <div className="bg-white border rounded-xl divide-y">
          {items.map((n) => (
            <div
              key={n.id}
              className={clsx(
                "px-4 py-3 flex gap-3 transition",
                !n.read_at && "bg-blue-50/40",
              )}
            >
              <span className="text-xl mt-0.5">{TYPE_ICON[n.type] ?? "📌"}</span>
              <button onClick={() => onClickItem(n)} className="flex-1 min-w-0 text-left hover:bg-gray-50 -mx-1 px-1 rounded">
                <div className="flex items-center gap-2">
                  <span className="font-medium">{n.title}</span>
                  {!n.read_at && <span className="w-2 h-2 bg-red-500 rounded-full" />}
                </div>
                {n.body && <p className="text-sm text-gray-600 mt-0.5">{n.body}</p>}
                <div className="text-[11px] text-gray-400 mt-1">{new Date(n.created_at).toLocaleString()}</div>
              </button>
              <button
                onClick={() => del.mutate(n.id)}
                className="text-gray-300 hover:text-red-500 text-sm self-start px-1"
                title="삭제"
              >
                ✕
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
