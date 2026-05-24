// API 응답 envelope (Design §6.2).

export type ApiEnvelope<T> = { data?: T; meta?: Record<string, unknown>; error?: ApiError };

export interface ApiError {
  code: string;
  message: string;
  rule_id?: string;
  details?: Record<string, unknown>;
  request_id?: string;
}

// 도메인 타입 (Go API와 1:1).

export type ShiftCode = "D" | "E" | "N" | "O" | "DE";
export type ScheduleStatus = "draft" | "generating" | "generated" | "confirmed" | "failed";
export type WishType = "off" | "d" | "e" | "n" | "unavailable";
export type FixedPattern = "D_ONLY" | "E_ONLY" | "N_ONLY" | "WEEKDAY_D" | "WEEKDAY_E";
export type SwapStatus =
  | "pending"
  | "b_accepted"
  | "approved"
  | "rejected_by_b"
  | "rejected_by_head"
  | "cancelled";

export interface Subject {
  nid: number;
  em: string;
  rl: "head_nurse" | "nurse";
}

export interface Nurse {
  id: number;
  name: string;
  role: "head_nurse" | "nurse";
  email?: string | null;             // Stage 1: nullable (계정 없이 명단만 추가 가능)
  hire_date?: string | null;
  experience_level_override?: string | null;
  fixed_shift_pattern?: FixedPattern | null;
  active: boolean;
  last_login_at?: string | null;
  created_at: string;
  resolved_level?: string;
}

export interface Level {
  id: number;
  code: string;
  display_name: string;
  min_months: number;
  max_months?: number | null;
  min_d: number;
  min_e: number;
  min_n: number;
  weight_coverage: number;
  weight_d_assignment: number;
  weight_e_assignment: number;
  weight_n_assignment: number;
  sort_order: number;
}

export interface Schedule {
  id: number;
  year_month: string;
  status: ScheduleStatus;
  generated_at?: string | null;
  confirmed_at?: string | null;
  generation_log?: GenerationLog | null;
}

export interface GenerationLog {
  solver_status?: string;
  violated_rule_ids?: string[];
  suggestion?: string;
  elapsed_ms?: number;
  applied_rules?: string[];
}

export interface ScheduleCell {
  id: number;
  schedule_id: number;
  nurse_id: number;
  date: string;        // YYYY-MM-DD
  shift: ShiftCode;
  source: "auto" | "manual";
  modified_by_nurse_id?: number | null;
  updated_at: string;
}

export interface Violation {
  rule_id: string;
  severity: "hard" | "soft";
  message: string;
  cell_ids?: number[];
  nurse_id?: number | null;
  date?: string | null;
}

export interface PatchCellMeta {
  violations: Violation[];
  hard_count: number;
  soft_count: number;
}

// ============================================================
// Notifications (Phase A)
// ============================================================

export type NotificationType =
  | "account_pending_approval"
  | "swap_request_received"
  | "swap_b_accepted"
  | "swap_approved"
  | "swap_rejected"
  | "level_changed"
  | "fixed_pattern_changed"
  | "nightkeeper_assigned"
  | "schedule_confirmed";

export interface Notification {
  id: number;
  type: NotificationType;
  title: string;
  body?: string;
  link?: string;
  meta?: Record<string, unknown>;
  read_at?: string | null;
  created_at: string;
}
