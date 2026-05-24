"""CP-SAT 모델 빌더 — 룰 ID 1:1 함수.

Design Ref: §9.2, §9.5 룰 매핑 / Rules v0.4

설계 원칙
- 순수 함수: I/O 없음, OR-Tools `CpModel`만 변형
- 룰 함수명 = 룰 ID (`H_01_no_n_to_d`, `K_01_nk_only_n_or_o` ...)
- ctx에 모든 입력·변수·메타·목적함수 항을 모음
- auto_mode=True: 자동 생성 (DE 도메인 제외, H-13)
- auto_mode=False: validate (DE 포함, 위반 식별만)
"""

from __future__ import annotations

import calendar
from collections.abc import Iterable
from dataclasses import dataclass, field
from datetime import date, timedelta
from typing import Any

from ortools.sat.python import cp_model

from .schemas import (
    CellOut,
    ExperienceLevelIn,
    GenerateInput,
    NurseIn,
    PrevCellIn,
    ShiftCode,
    WardSettingsIn,
    WishIn,
)

# ============================================================
# Constants
# ============================================================

WORK_SHIFTS: tuple[ShiftCode, ...] = ("D", "E", "N", "DE")
ALL_SHIFTS: tuple[ShiftCode, ...] = ("D", "E", "N", "O", "DE")
AUTO_SHIFTS: tuple[ShiftCode, ...] = ("D", "E", "N", "O")  # H-13: DE 제외


def is_d_count(shift: ShiftCode) -> bool:
    """G-05: D와 DE는 D 카운트에 동시 포함."""
    return shift in ("D", "DE")


def is_e_count(shift: ShiftCode) -> bool:
    """G-05: E와 DE는 E 카운트에 동시 포함."""
    return shift in ("E", "DE")


def is_n_count(shift: ShiftCode) -> bool:
    """N 카운트 (DE는 N 아님)."""
    return shift == "N"


def is_work(shift: ShiftCode) -> bool:
    return shift in WORK_SHIFTS


# ============================================================
# Context
# ============================================================


@dataclass
class Ctx:
    """솔버 호출 컨텍스트. 룰 함수들이 공유."""

    nurses: list[NurseIn]
    dates: list[date]                                # 이번 달 (변수)
    prev_dates: list[date]                           # 이전 달 (상수)
    timeline: list[date]                             # prev + curr 정렬
    prev_cell_map: dict[tuple[int, date], ShiftCode] # (nurse_id, date) -> shift
    wishes: list[WishIn]
    levels: dict[str, ExperienceLevelIn]
    nurse_level: dict[int, str]                      # nurse_id -> level code
    settings: WardSettingsIn
    auto_mode: bool = True
    domain_shifts: tuple[ShiftCode, ...] = AUTO_SHIFTS

    # 결정 변수: x[(nurse_id, date, shift)] -> BoolVar (curr only)
    x: dict[tuple[int, date, ShiftCode], cp_model.IntVar] = field(default_factory=dict)
    # 목적함수 항
    obj_terms: list[Any] = field(default_factory=list)
    applied_rules: list[str] = field(default_factory=list)

    def effective(
        self, nurse_id: int, t: date, s: ShiftCode
    ) -> int | cp_model.IntVar:
        """timeline 위치별로 상수 또는 변수 반환."""
        if (nurse_id, t) in self.prev_cell_map:
            return 1 if self.prev_cell_map[(nurse_id, t)] == s else 0
        if (nurse_id, t, s) in self.x:
            return self.x[(nurse_id, t, s)]
        return 0


# ============================================================
# Context builder
# ============================================================


def _month_dates(year_month: str) -> list[date]:
    y, m = map(int, year_month.split("-"))
    _, days = calendar.monthrange(y, m)
    return [date(y, m, d) for d in range(1, days + 1)]


def _prev_dates(first_curr: date, lookback: int) -> list[date]:
    return [first_curr - timedelta(days=k) for k in range(lookback, 0, -1)]


def build_context(inp: GenerateInput, *, auto_mode: bool = True) -> Ctx:
    dates = _month_dates(inp.year_month)
    prev_dates = _prev_dates(dates[0], lookback=7)  # ward_settings.previous_month_lookback_days
    timeline = prev_dates + dates

    prev_cell_map = {(c.nurse_id, c.date): c.shift for c in inp.previous_month_last_week_cells}

    levels = {lv.code: lv for lv in inp.experience_levels}
    nurse_level = {n.id: n.level for n in inp.nurses}

    domain = AUTO_SHIFTS if auto_mode else ALL_SHIFTS

    return Ctx(
        nurses=inp.nurses,
        dates=dates,
        prev_dates=prev_dates,
        timeline=timeline,
        prev_cell_map=prev_cell_map,
        wishes=inp.wishes,
        levels=levels,
        nurse_level=nurse_level,
        settings=inp.ward_settings,
        auto_mode=auto_mode,
        domain_shifts=domain,
    )


# ============================================================
# Variables + Base Constraints
# ============================================================


def declare_variables(model: cp_model.CpModel, ctx: Ctx) -> None:
    """결정 변수 생성. (nurse_id, date, shift) -> BoolVar."""
    for n in ctx.nurses:
        for t in ctx.dates:
            for s in ctx.domain_shifts:
                ctx.x[(n.id, t, s)] = model.NewBoolVar(f"x_{n.id}_{t.isoformat()}_{s}")


def G_02_one_shift_per_day(model: cp_model.CpModel, ctx: Ctx) -> None:
    """G-02: 1인 1일 1시프트."""
    for n in ctx.nurses:
        for t in ctx.dates:
            model.AddExactlyOne(ctx.x[(n.id, t, s)] for s in ctx.domain_shifts)
    ctx.applied_rules.append("G-02")


# ============================================================
# HARD RULES — H_NN
# ============================================================


def H_01_no_n_to_d(model: cp_model.CpModel, ctx: Ctx) -> None:
    """H-01: N(t) → D(t+1) 금지. H-10: 이전 달 마지막날→이번 달 1일도 적용."""
    for n in ctx.nurses:
        for i, t in enumerate(ctx.timeline[:-1]):
            t_next = ctx.timeline[i + 1]
            n_var = ctx.effective(n.id, t, "N")
            d_var = ctx.effective(n.id, t_next, "D")
            if isinstance(n_var, int) and n_var == 0:
                continue
            if isinstance(d_var, int) and d_var == 0:
                continue
            model.Add(n_var + d_var <= 1)
    ctx.applied_rules.append("H-01")


def H_02_max_consecutive_n(model: cp_model.CpModel, ctx: Ctx) -> None:
    """H-02: N 연속 max_consecutive_n. 슬라이딩 윈도우 (max+1 일에 N이 max 이하)."""
    nmax = ctx.settings.max_consecutive_n
    window = nmax + 1
    for n in ctx.nurses:
        if len(ctx.timeline) < window:
            continue
        for start in range(len(ctx.timeline) - window + 1):
            ts = ctx.timeline[start : start + window]
            # 검사 윈도우가 이번 달 변수를 적어도 하나 포함해야 의미 있음
            if not any(t in ctx.dates for t in ts):
                continue
            model.Add(sum(_to_expr(ctx.effective(n.id, t, "N")) for t in ts) <= nmax)
    ctx.applied_rules.append("H-02")


def H_03_max_workdays(model: cp_model.CpModel, ctx: Ctx) -> None:
    """H-03: 연속 근무 max_consecutive_workdays. Off 없이 work_shift 합 ≤ max."""
    wmax = ctx.settings.max_consecutive_workdays
    window = wmax + 1
    for n in ctx.nurses:
        if len(ctx.timeline) < window:
            continue
        for start in range(len(ctx.timeline) - window + 1):
            ts = ctx.timeline[start : start + window]
            if not any(t in ctx.dates for t in ts):
                continue
            terms = []
            for t in ts:
                for s in WORK_SHIFTS:
                    terms.append(_to_expr(ctx.effective(n.id, t, s)))
            model.Add(sum(terms) <= wmax)
    ctx.applied_rules.append("H-03")


def H_04_shift_min(model: cp_model.CpModel, ctx: Ctx) -> None:
    """H-04: 시프트별 전체 최소 인원. DE는 D·E 카운트에 동시 (G-05)."""
    s_def = (("D", ctx.settings.min_d), ("E", ctx.settings.min_e), ("N", ctx.settings.min_n))
    for t in ctx.dates:
        for shift, mn in s_def:
            if mn <= 0:
                continue
            terms = []
            for n in ctx.nurses:
                # primary
                terms.append(_to_expr(ctx.effective(n.id, t, shift)))
                # DE는 D·E에 카운트 (auto_mode면 DE 없으니 자연스럽게 0)
                if shift in ("D", "E"):
                    terms.append(_to_expr(ctx.effective(n.id, t, "DE")))
            model.Add(sum(terms) >= mn)
    ctx.applied_rules.append("H-04")


def H_05_unavailable(model: cp_model.CpModel, ctx: Ctx) -> None:
    """H-05: unavailable 희망 시 O 강제 (즉 work 시프트 0)."""
    for w in ctx.wishes:
        if w.type != "unavailable":
            continue
        if w.date not in ctx.dates:
            continue
        for s in WORK_SHIFTS:
            if (w.nurse_id, w.date, s) in ctx.x:
                model.Add(ctx.x[(w.nurse_id, w.date, s)] == 0)
    ctx.applied_rules.append("H-05")


def H_06_rest_after_n(model: cp_model.CpModel, ctx: Ctx) -> None:
    """H-06: N(t) → O(t+1) 최소 1일."""
    rest = ctx.settings.min_rest_after_n
    for n in ctx.nurses:
        for i, t in enumerate(ctx.timeline[: -1]):
            n_var = ctx.effective(n.id, t, "N")
            if isinstance(n_var, int) and n_var == 0:
                continue
            for k in range(1, rest + 1):
                idx = i + k
                if idx >= len(ctx.timeline):
                    break
                t_after = ctx.timeline[idx]
                if t_after not in ctx.dates:
                    continue
                o_var = ctx.effective(n.id, t_after, "O")
                # n_var = 1 implies o_var = 1
                # n_var <= o_var
                model.Add(_to_expr(n_var) <= _to_expr(o_var))
    ctx.applied_rules.append("H-06")


def H_10_month_boundary(model: cp_model.CpModel, ctx: Ctx) -> None:
    """H-10: 이전 달 마지막 7일을 상수로 timeline에 포함.
    실제 효과는 H-01/H-02/H-03/H-06이 timeline 위에서 동작하므로 자동.
    이 함수는 'applied_rules' 마커만 추가."""
    ctx.applied_rules.append("H-10")


def H_11_fixed_pattern(model: cp_model.CpModel, ctx: Ctx) -> None:
    """H-11: fixed_shift_pattern 설정자는 패턴대로만."""
    for n in ctx.nurses:
        if not n.fixed_pattern:
            continue
        allowed = _fixed_pattern_allowed_shifts(n.fixed_pattern, dates=ctx.dates)
        for t in ctx.dates:
            for s in ctx.domain_shifts:
                if (n.id, t, s) in ctx.x:
                    if s not in allowed.get(t, set()):
                        model.Add(ctx.x[(n.id, t, s)] == 0)
    ctx.applied_rules.append("H-11")


def H_12_level_per_shift(model: cp_model.CpModel, ctx: Ctx) -> None:
    """H-12: 시프트당 등급별 최소 인원."""
    for t in ctx.dates:
        for shift, attr in (("D", "min_d"), ("E", "min_e"), ("N", "min_n")):
            for code, lv in ctx.levels.items():
                mn = getattr(lv, attr)
                if mn <= 0:
                    continue
                terms = []
                for n in ctx.nurses:
                    if ctx.nurse_level.get(n.id) != code:
                        continue
                    terms.append(_to_expr(ctx.effective(n.id, t, shift)))
                    if shift in ("D", "E"):
                        terms.append(_to_expr(ctx.effective(n.id, t, "DE")))
                if terms:
                    model.Add(sum(terms) >= mn)
    ctx.applied_rules.append("H-12")


def H_13_exclude_de_auto(model: cp_model.CpModel, ctx: Ctx) -> None:
    """H-13: 자동 생성 모드에선 DE 도메인 제외 — declare_variables에서 이미 적용.
    이 함수는 마커만 추가."""
    if ctx.auto_mode:
        ctx.applied_rules.append("H-13")


def H_14_de_then_no_d(model: cp_model.CpModel, ctx: Ctx) -> None:
    """H-14: DE(t) → D(t+1) 금지. auto_mode면 DE가 결정변수가 아니므로 prev_cells만 영향."""
    for n in ctx.nurses:
        for i, t in enumerate(ctx.timeline[:-1]):
            t_next = ctx.timeline[i + 1]
            de_var = ctx.effective(n.id, t, "DE")
            d_var = ctx.effective(n.id, t_next, "D")
            if isinstance(de_var, int) and de_var == 0:
                continue
            if isinstance(d_var, int) and d_var == 0:
                continue
            model.Add(_to_expr(de_var) + _to_expr(d_var) <= 1)
    ctx.applied_rules.append("H-14")


# ============================================================
# K-NN  Night-Keeper
# ============================================================


def K_01_nk_only_n_or_o(model: cp_model.CpModel, ctx: Ctx) -> None:
    """K-01: night_keeper 지정자는 N/O만."""
    for n in ctx.nurses:
        if not n.is_night_keeper:
            continue
        for t in ctx.dates:
            for s in ctx.domain_shifts:
                if s in ("D", "E", "DE") and (n.id, t, s) in ctx.x:
                    model.Add(ctx.x[(n.id, t, s)] == 0)
    ctx.applied_rules.append("K-01")


# ============================================================
# SOFT RULES — S_NN  (목적함수 항)
# ============================================================


def S_01_balance_off(model: cp_model.CpModel, ctx: Ctx) -> None:
    """S-01: 월 Off 수 균형 — 평균과의 편차(절댓값)의 합 * weight."""
    w = ctx.settings.weight_balance_off
    if w <= 0:
        return
    # off_count[i] = Σ_t x[i, t, 'O']
    off_counts: dict[int, cp_model.IntVar] = {}
    max_days = len(ctx.dates)
    for n in ctx.nurses:
        v = model.NewIntVar(0, max_days, f"off_cnt_{n.id}")
        model.Add(v == sum(ctx.x[(n.id, t, "O")] for t in ctx.dates))
        off_counts[n.id] = v

    # avg는 정수가 아닐 수 있으므로 N*Σ = Σ_i (off_i) 이용
    # 편차 abs를 abs aux var로 표현
    total = sum(off_counts.values())
    num = len(ctx.nurses)
    if num == 0:
        return
    # 각 nurse: dev_i = num * off_i - total  (= num*(off_i - mean))
    # 목적: Σ |dev_i|
    for nid, oc in off_counts.items():
        dev = model.NewIntVar(-num * max_days, num * max_days, f"off_dev_{nid}")
        model.Add(dev == num * oc - total)
        abs_dev = model.NewIntVar(0, num * max_days, f"off_abs_{nid}")
        model.AddAbsEquality(abs_dev, dev)
        ctx.obj_terms.append(w * abs_dev)
    ctx.applied_rules.append("S-01")


def S_02_respect_wishes(model: cp_model.CpModel, ctx: Ctx) -> None:
    """S-02: 희망일(off/d/e/n) 미반영 시 페널티."""
    w = ctx.settings.weight_respect_wishes
    if w <= 0:
        return
    wish_to_shift: dict[str, ShiftCode] = {"off": "O", "d": "D", "e": "E", "n": "N"}
    for ws in ctx.wishes:
        if ws.type not in wish_to_shift:
            continue
        if ws.date not in ctx.dates:
            continue
        s = wish_to_shift[ws.type]
        if (ws.nurse_id, ws.date, s) not in ctx.x:
            continue
        # not_satisfied = 1 - x
        ctx.obj_terms.append(w * (1 - ctx.x[(ws.nurse_id, ws.date, s)]))
    ctx.applied_rules.append("S-02")


def S_03_weekend_balance(model: cp_model.CpModel, ctx: Ctx) -> None:
    """S-03: 주말(토·일) 근무 편차. G-06: 공휴일 별도 처리 X."""
    w = ctx.settings.weight_weekend_balance
    if w <= 0:
        return
    weekend = [t for t in ctx.dates if t.weekday() >= 5]
    if not weekend or not ctx.nurses:
        return
    cnt: dict[int, cp_model.IntVar] = {}
    for n in ctx.nurses:
        v = model.NewIntVar(0, len(weekend), f"wk_cnt_{n.id}")
        terms = []
        for t in weekend:
            for s in WORK_SHIFTS:
                if (n.id, t, s) in ctx.x:
                    terms.append(ctx.x[(n.id, t, s)])
        if terms:
            model.Add(v == sum(terms))
        else:
            model.Add(v == 0)
        cnt[n.id] = v

    total = sum(cnt.values())
    num = len(ctx.nurses)
    max_w = len(weekend)
    for nid, c in cnt.items():
        dev = model.NewIntVar(-num * max_w, num * max_w, f"wk_dev_{nid}")
        model.Add(dev == num * c - total)
        ab = model.NewIntVar(0, num * max_w, f"wk_abs_{nid}")
        model.AddAbsEquality(ab, dev)
        ctx.obj_terms.append(w * ab)
    ctx.applied_rules.append("S-03")


def S_04_same_shift_streak(model: cp_model.CpModel, ctx: Ctx) -> None:
    """S-04: D 또는 E 3연속 이상 페널티. 4일 윈도우에서 D 4건/E 4건이면 페널티 1."""
    w = ctx.settings.weight_same_shift_streak
    if w <= 0:
        return
    window = 4
    for n in ctx.nurses:
        if len(ctx.dates) < window:
            continue
        for shift in ("D", "E"):
            for start in range(len(ctx.dates) - window + 1):
                ts = ctx.dates[start : start + window]
                same = []
                for t in ts:
                    if (n.id, t, shift) in ctx.x:
                        same.append(ctx.x[(n.id, t, shift)])
                    # DE도 D 또는 E 카운트
                    if shift == "D" and (n.id, t, "DE") in ctx.x:
                        same.append(ctx.x[(n.id, t, "DE")])
                    if shift == "E" and (n.id, t, "DE") in ctx.x:
                        same.append(ctx.x[(n.id, t, "DE")])
                if len(same) < window:
                    continue
                # all 4 same shift = sum == 4 → over var = max(0, sum - 3)
                over = model.NewIntVar(0, window, f"streak_{n.id}_{shift}_{start}")
                model.Add(over >= sum(same) - (window - 1))
                ctx.obj_terms.append(w * over)
    ctx.applied_rules.append("S-04")


def S_05_short_rest(model: cp_model.CpModel, ctx: Ctx) -> None:
    """S-05: N → O → D 짧은 휴식 패턴 페널티 (H-06 hard보다 강한 soft 권장).
    감지: x[i,t,N]=1 ∧ x[i,t+1,O]=1 ∧ x[i,t+2,D]=1 → 페널티 1."""
    w = ctx.settings.weight_short_rest_pattern
    if w <= 0:
        return
    for n in ctx.nurses:
        for i in range(len(ctx.timeline) - 2):
            t0, t1, t2 = ctx.timeline[i], ctx.timeline[i + 1], ctx.timeline[i + 2]
            # 의미 있는 윈도우는 t2가 dates에 있을 때
            if t2 not in ctx.dates:
                continue
            n_e = ctx.effective(n.id, t0, "N")
            o_e = ctx.effective(n.id, t1, "O")
            d_e = ctx.effective(n.id, t2, "D")
            # 상수 단순화
            if isinstance(n_e, int) and n_e == 0:
                continue
            if isinstance(o_e, int) and o_e == 0:
                continue
            if isinstance(d_e, int) and d_e == 0:
                continue
            # pattern_var = AND(n, o, d). 페널티는 pattern_var.
            pv = model.NewBoolVar(f"short_rest_{n.id}_{t0}")
            model.AddBoolAnd([_to_bool(model, n_e), _to_bool(model, o_e), _to_bool(model, d_e)]).OnlyEnforceIf(pv)
            model.AddBoolOr([_neg(model, _to_bool(model, n_e)), _neg(model, _to_bool(model, o_e)), _neg(model, _to_bool(model, d_e))]).OnlyEnforceIf(pv.Not())
            ctx.obj_terms.append(w * pv)
    ctx.applied_rules.append("S-05")


def S_10_level_assignment_cost(model: cp_model.CpModel, ctx: Ctx) -> None:
    """S-10: 등급별 시프트 배정 비용 (weight_d/e/n_assignment)."""
    has_any = False
    for code, lv in ctx.levels.items():
        for shift, wt in (
            ("D", lv.weight_d_assignment),
            ("E", lv.weight_e_assignment),
            ("N", lv.weight_n_assignment),
        ):
            if wt <= 0:
                continue
            has_any = True
            for n in ctx.nurses:
                if ctx.nurse_level.get(n.id) != code:
                    continue
                for t in ctx.dates:
                    if (n.id, t, shift) in ctx.x:
                        ctx.obj_terms.append(wt * ctx.x[(n.id, t, shift)])
    if has_any:
        ctx.applied_rules.append("S-10")


# ============================================================
# Helpers
# ============================================================


def _to_expr(v: int | cp_model.IntVar) -> Any:
    return v


def _to_bool(model: cp_model.CpModel, v: int | cp_model.IntVar) -> cp_model.IntVar:
    """상수 정수를 BoolVar로 승격 (S-05 같은 AND 구성용)."""
    if isinstance(v, int):
        b = model.NewBoolVar(f"const_{v}")
        model.Add(b == v)
        return b
    return v


def _neg(model: cp_model.CpModel, b: cp_model.IntVar) -> cp_model.IntVar:
    return b.Not()


def _fixed_pattern_allowed_shifts(
    pat: str, dates: Iterable[date]
) -> dict[date, set[ShiftCode]]:
    """H-11: 패턴별 t마다 허용되는 시프트 집합 계산."""
    result: dict[date, set[ShiftCode]] = {}
    for t in dates:
        is_weekend = t.weekday() >= 5  # G-06: 공휴일 별도 처리 X
        if pat == "D_ONLY":
            result[t] = {"D", "O"}
        elif pat == "E_ONLY":
            result[t] = {"E", "O"}
        elif pat == "N_ONLY":
            result[t] = {"N", "O"}
        elif pat == "WEEKDAY_D":
            result[t] = {"O"} if is_weekend else {"D", "O"}
        elif pat == "WEEKDAY_E":
            result[t] = {"O"} if is_weekend else {"E", "O"}
        else:
            result[t] = set(ALL_SHIFTS)
    return result


# ============================================================
# Build entrypoint
# ============================================================


HARD_RULES = (
    G_02_one_shift_per_day,
    H_01_no_n_to_d,
    H_02_max_consecutive_n,
    H_03_max_workdays,
    H_04_shift_min,
    H_05_unavailable,
    H_06_rest_after_n,
    H_10_month_boundary,
    H_11_fixed_pattern,
    H_12_level_per_shift,
    H_13_exclude_de_auto,
    H_14_de_then_no_d,
    K_01_nk_only_n_or_o,
)

SOFT_RULES = (
    S_01_balance_off,
    S_02_respect_wishes,
    S_03_weekend_balance,
    S_04_same_shift_streak,
    S_05_short_rest,
    S_10_level_assignment_cost,
)


def build_model(inp: GenerateInput, *, auto_mode: bool = True) -> tuple[cp_model.CpModel, Ctx]:
    """CP-SAT 모델을 빌드해 반환. 순수 함수 (I/O 없음)."""
    ctx = build_context(inp, auto_mode=auto_mode)
    model = cp_model.CpModel()

    declare_variables(model, ctx)
    for rule in HARD_RULES:
        rule(model, ctx)
    for rule in SOFT_RULES:
        rule(model, ctx)

    if ctx.obj_terms:
        model.Minimize(sum(ctx.obj_terms))

    return model, ctx


def solve_and_extract(
    inp: GenerateInput, *, max_time_seconds: int | None = None
) -> tuple[str, int | None, list[CellOut], int, list[str]]:
    """모델 빌드 → 풀이 → cells 추출.

    Returns
    -------
    (solver_status_name, objective_value or None, cells, elapsed_ms, applied_rules)
    """
    import time

    model, ctx = build_model(inp, auto_mode=True)
    solver = cp_model.CpSolver()
    if max_time_seconds is None:
        max_time_seconds = inp.max_time_seconds
    solver.parameters.max_time_in_seconds = float(max_time_seconds)

    t0 = time.perf_counter()
    status = solver.Solve(model)
    elapsed_ms = int((time.perf_counter() - t0) * 1000)
    status_name = solver.StatusName(status)

    if status not in (cp_model.OPTIMAL, cp_model.FEASIBLE):
        return status_name, None, [], elapsed_ms, ctx.applied_rules

    cells: list[CellOut] = []
    for (nid, t, s), var in ctx.x.items():
        if solver.Value(var) == 1:
            cells.append(CellOut(nurse_id=nid, date=t, shift=s))
    cells.sort(key=lambda c: (c.date, c.nurse_id))
    obj = int(solver.ObjectiveValue()) if ctx.obj_terms else 0
    return status_name, obj, cells, elapsed_ms, ctx.applied_rules
