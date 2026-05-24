"""주어진 cells가 hard/soft 룰을 만족하는지 검사 (수동 수정 후 호출).

Design Ref: §4.3 /validate, §9.5 룰 매핑
"""

from __future__ import annotations

import calendar
from collections import defaultdict
from datetime import date, timedelta

from .model import (
    WORK_SHIFTS,
    _fixed_pattern_allowed_shifts,
    is_d_count,
    is_e_count,
)
from .schemas import (
    CellOut,
    ExperienceLevelIn,
    NurseIn,
    PrevCellIn,
    ShiftCode,
    ValidateInput,
    Violation,
    WardSettingsIn,
    WishIn,
)


def validate(inp: ValidateInput) -> list[Violation]:
    """모든 hard/soft 룰을 검사해 위반 목록 반환."""
    violations: list[Violation] = []

    # 인덱스
    y, m = map(int, inp.year_month.split("-"))
    _, days = calendar.monthrange(y, m)
    dates = [date(y, m, d) for d in range(1, days + 1)]
    first = dates[0]
    prev_dates = [first - timedelta(days=k) for k in range(7, 0, -1)]
    timeline = prev_dates + dates

    # nurse_id, date -> shift (current + prev 합쳐서)
    shift_at: dict[tuple[int, date], ShiftCode] = {}
    for c in inp.previous_month_last_week_cells:
        shift_at[(c.nurse_id, c.date)] = c.shift
    for c in inp.all_cells:
        shift_at[(c.nurse_id, c.date)] = c.shift

    # nurse map
    nurse_map = {n.id: n for n in inp.nurses}
    level_map = {lv.code: lv for lv in inp.experience_levels}

    # ----- HARD -----
    _check_H_01(shift_at, nurse_map, timeline, dates, violations)
    _check_H_02(shift_at, nurse_map, timeline, dates, inp.ward_settings, violations)
    _check_H_03(shift_at, nurse_map, timeline, dates, inp.ward_settings, violations)
    _check_H_04(shift_at, inp.nurses, dates, inp.ward_settings, violations)
    _check_H_05(shift_at, inp.wishes, dates, violations)
    _check_H_06(shift_at, nurse_map, timeline, dates, inp.ward_settings, violations)
    _check_H_11(shift_at, inp.nurses, dates, violations)
    _check_H_12(shift_at, inp.nurses, level_map, dates, violations)
    _check_H_14(shift_at, nurse_map, timeline, dates, violations)
    _check_K_01(shift_at, inp.nurses, dates, violations)

    return violations


# ============================================================
# Hard checkers
# ============================================================


def _check_H_01(shift_at, nurse_map, timeline, curr_dates, violations):
    for nid in nurse_map:
        for i, t in enumerate(timeline[:-1]):
            t_next = timeline[i + 1]
            if shift_at.get((nid, t)) == "N" and shift_at.get((nid, t_next)) == "D":
                if t_next in curr_dates:
                    violations.append(
                        Violation(
                            rule_id="H-01",
                            severity="hard",
                            message="N 다음날 D 금지",
                            nurse_id=nid,
                            date=t_next,
                        )
                    )


def _check_H_02(shift_at, nurse_map, timeline, curr_dates, settings: WardSettingsIn, violations):
    nmax = settings.max_consecutive_n
    window = nmax + 1
    for nid in nurse_map:
        if len(timeline) < window:
            continue
        for start in range(len(timeline) - window + 1):
            ts = timeline[start : start + window]
            cnt = sum(1 for t in ts if shift_at.get((nid, t)) == "N")
            if cnt > nmax and any(t in curr_dates for t in ts):
                violations.append(
                    Violation(
                        rule_id="H-02",
                        severity="hard",
                        message=f"N 연속 {nmax + 1}일 초과",
                        nurse_id=nid,
                        date=ts[-1],
                    )
                )
                break  # 같은 nurse는 한 번만 보고


def _check_H_03(shift_at, nurse_map, timeline, curr_dates, settings: WardSettingsIn, violations):
    wmax = settings.max_consecutive_workdays
    window = wmax + 1
    for nid in nurse_map:
        if len(timeline) < window:
            continue
        for start in range(len(timeline) - window + 1):
            ts = timeline[start : start + window]
            cnt = sum(1 for t in ts if shift_at.get((nid, t)) in WORK_SHIFTS)
            if cnt > wmax and any(t in curr_dates for t in ts):
                violations.append(
                    Violation(
                        rule_id="H-03",
                        severity="hard",
                        message=f"연속 근무 {wmax + 1}일 초과",
                        nurse_id=nid,
                        date=ts[-1],
                    )
                )
                break


def _check_H_04(shift_at, nurses: list[NurseIn], dates, settings: WardSettingsIn, violations):
    for t in dates:
        d_cnt = sum(1 for n in nurses if is_d_count(shift_at.get((n.id, t), "O")))
        e_cnt = sum(1 for n in nurses if is_e_count(shift_at.get((n.id, t), "O")))
        n_cnt = sum(1 for n in nurses if shift_at.get((n.id, t)) == "N")
        if d_cnt < settings.min_d:
            violations.append(
                Violation(rule_id="H-04", severity="hard",
                          message=f"D 최소 인원 미달 ({d_cnt}/{settings.min_d})", date=t)
            )
        if e_cnt < settings.min_e:
            violations.append(
                Violation(rule_id="H-04", severity="hard",
                          message=f"E 최소 인원 미달 ({e_cnt}/{settings.min_e})", date=t)
            )
        if n_cnt < settings.min_n:
            violations.append(
                Violation(rule_id="H-04", severity="hard",
                          message=f"N 최소 인원 미달 ({n_cnt}/{settings.min_n})", date=t)
            )


def _check_H_05(shift_at, wishes: list[WishIn], dates, violations):
    for w in wishes:
        if w.type != "unavailable" or w.date not in dates:
            continue
        s = shift_at.get((w.nurse_id, w.date))
        if s and s != "O":
            violations.append(
                Violation(rule_id="H-05", severity="hard",
                          message="불가일에 배정됨", nurse_id=w.nurse_id, date=w.date)
            )


def _check_H_06(shift_at, nurse_map, timeline, curr_dates, settings: WardSettingsIn, violations):
    rest = settings.min_rest_after_n
    for nid in nurse_map:
        for i, t in enumerate(timeline[:-1]):
            if shift_at.get((nid, t)) != "N":
                continue
            for k in range(1, rest + 1):
                idx = i + k
                if idx >= len(timeline):
                    break
                t_after = timeline[idx]
                if t_after not in curr_dates:
                    continue
                if shift_at.get((nid, t_after)) != "O":
                    violations.append(
                        Violation(rule_id="H-06", severity="hard",
                                  message=f"N 후 휴식 부족 ({k}일 차)",
                                  nurse_id=nid, date=t_after)
                    )
                    break


def _check_H_11(shift_at, nurses: list[NurseIn], dates, violations):
    for n in nurses:
        if not n.fixed_pattern:
            continue
        allowed = _fixed_pattern_allowed_shifts(n.fixed_pattern, dates=dates)
        for t in dates:
            s = shift_at.get((n.id, t))
            if s and s not in allowed.get(t, set()):
                violations.append(
                    Violation(rule_id="H-11", severity="hard",
                              message=f"고정 패턴({n.fixed_pattern}) 위반: {s}",
                              nurse_id=n.id, date=t)
                )


def _check_H_12(
    shift_at, nurses: list[NurseIn], levels: dict[str, ExperienceLevelIn], dates, violations
):
    by_level: dict[str, list[int]] = defaultdict(list)
    for n in nurses:
        by_level[n.level].append(n.id)
    for t in dates:
        for code, lv in levels.items():
            nids = by_level.get(code, [])
            d_cnt = sum(1 for nid in nids if is_d_count(shift_at.get((nid, t), "O")))
            e_cnt = sum(1 for nid in nids if is_e_count(shift_at.get((nid, t), "O")))
            n_cnt = sum(1 for nid in nids if shift_at.get((nid, t)) == "N")
            if lv.min_d > 0 and d_cnt < lv.min_d:
                violations.append(
                    Violation(rule_id="H-12", severity="hard",
                              message=f"{code} 등급 D 최소 미달 ({d_cnt}/{lv.min_d})", date=t)
                )
            if lv.min_e > 0 and e_cnt < lv.min_e:
                violations.append(
                    Violation(rule_id="H-12", severity="hard",
                              message=f"{code} 등급 E 최소 미달 ({e_cnt}/{lv.min_e})", date=t)
                )
            if lv.min_n > 0 and n_cnt < lv.min_n:
                violations.append(
                    Violation(rule_id="H-12", severity="hard",
                              message=f"{code} 등급 N 최소 미달 ({n_cnt}/{lv.min_n})", date=t)
                )


def _check_H_14(shift_at, nurse_map, timeline, curr_dates, violations):
    for nid in nurse_map:
        for i, t in enumerate(timeline[:-1]):
            t_next = timeline[i + 1]
            if shift_at.get((nid, t)) == "DE" and shift_at.get((nid, t_next)) == "D":
                if t_next in curr_dates:
                    violations.append(
                        Violation(rule_id="H-14", severity="hard",
                                  message="DE 다음날 D 금지",
                                  nurse_id=nid, date=t_next)
                    )


def _check_K_01(shift_at, nurses: list[NurseIn], dates, violations):
    for n in nurses:
        if not n.is_night_keeper:
            continue
        for t in dates:
            s = shift_at.get((n.id, t))
            if s in ("D", "E", "DE"):
                violations.append(
                    Violation(rule_id="K-01", severity="hard",
                              message="나이트킵은 N/O만 가능",
                              nurse_id=n.id, date=t)
                )
