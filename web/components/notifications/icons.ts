// 알림 타입 → lucide 아이콘 + 색상 매핑.
// NotificationBell, /notifications 페이지 양쪽에서 공유.
import {
  UserPlus,
  ArrowLeftRight,
  Handshake,
  CheckCircle2,
  XCircle,
  Tag,
  Settings,
  Moon,
  CalendarCheck,
  Bell,
  type LucideIcon,
} from "lucide-react";

export interface NotifIconSpec {
  Icon: LucideIcon;
  color: string; // tailwind text color class
}

export const NOTIF_ICON: Record<string, NotifIconSpec> = {
  account_pending_approval: { Icon: UserPlus,        color: "text-amber-600"   },
  swap_request_received:    { Icon: ArrowLeftRight,  color: "text-blue-600"    },
  swap_b_accepted:          { Icon: Handshake,       color: "text-green-600"   },
  swap_approved:            { Icon: CheckCircle2,    color: "text-emerald-600" },
  swap_rejected:            { Icon: XCircle,         color: "text-red-500"     },
  level_changed:            { Icon: Tag,             color: "text-purple-600"  },
  fixed_pattern_changed:    { Icon: Settings,        color: "text-gray-600"    },
  nightkeeper_assigned:     { Icon: Moon,            color: "text-indigo-600"  },
  schedule_confirmed:       { Icon: CalendarCheck,   color: "text-emerald-600" },
};

export const FALLBACK_NOTIF_ICON: NotifIconSpec = { Icon: Bell, color: "text-gray-500" };

export function getNotifIcon(type: string): NotifIconSpec {
  return NOTIF_ICON[type] ?? FALLBACK_NOTIF_ICON;
}
