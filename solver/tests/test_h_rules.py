"""L0 — Hard rule unit tests (H_NN).

각 테스트는 솔버를 풀어서 결과 cells에 해당 룰 위반이 없는지 검증.
일부는 validator로 의도적 위반 입력을 검증.
"""

from __future__ import annotations

from datetime import date, timedelta

import pytest

from app.model import WORK_SHIFTS, solve_and_extract
from app.schemas import (
    CellOut,
    ExperienceLevelIn,
    GenerateInput,
    NurseIn,
    PrevCellIn,
    ValidateInput,
    WardSettingsIn,
    WishIn,
)
from app.validator import validate
from tests.conftest import make_small_input


# ============================================================
# H-01: N → D 금지
# ============================================================


def test_h01_no_n_to_d(simple_input):
    status, _, cells, _, applied = solve_and_extract(simple_input)
    assert status in ("OPTIMAL", "FEASIBLE")
    assert "H-01" in applied
    by_nd = {(c.nurse_id, c.date): c.shift for c in cells}
    for (nid, d), s in by_nd.items():
        if s == "N":
            assert by_nd.get((nid, d + timedelta(days=1))) != "D", \
                f"H-01 violated at nurse={nid} date={d}"


def test_h01_validator_detects_violation():
    """수동 수정으로 N→D 만들면 validate가 잡아야 함."""
    inp = make_small_input(n_count=3, min_d=1, min_e=0, min_n=0)
    cells = [
        CellOut(nurse_id=1, date=date(2026, 6, 1), shift="N"),
        CellOut(nurse_id=1, date=date(2026, 6, 2), shift="D"),  # 위반
    ]
    # 나머지 nurse는 어떻게든 채워야 unique 위반 안 남
    for nid in (2, 3):
        for d in range(1, 31):
            cells.append(CellOut(nurse_id=nid, date=date(2026, 6, d), shift="O"))
    for d in range(3, 31):
        cells.append(CellOut(nurse_id=1, date=date(2026, 6, d), shift="O"))

    vinp = ValidateInput(
        year_month="2026-06",
        nurses=inp.nurses,
        wishes=[],
        previous_month_last_week_cells=[],
        experience_levels=inp.experience_levels,
        ward_settings=inp.ward_settings,
        all_cells=cells,
    )
    vs = validate(vinp)
    assert any(v.rule_id == "H-01" for v in vs)


# ============================================================
# H-02: N 연속 max
# ============================================================


def test_h02_max_consecutive_n(simple_input):
    inp = simple_input.model_copy(update={"ward_settings": simple_input.ward_settings.model_copy(update={"max_consecutive_n": 2})})
    status, _, cells, _, applied = solve_and_extract(inp)
    assert status in ("OPTIMAL", "FEASIBLE")
    by = {(c.nurse_id, c.date): c.shift for c in cells}
    nurse_ids = {n.id for n in inp.nurses}
    for nid in nurse_ids:
        streak = 0
        # date 기준 정렬
        dates_sorted = sorted({c.date for c in cells})
        for d in dates_sorted:
            if by.get((nid, d)) == "N":
                streak += 1
                assert streak <= 2, f"H-02: nurse {nid} N streak {streak}"
            else:
                streak = 0


# ============================================================
# H-03: 연속 근무 5일
# ============================================================


def test_h03_max_workdays():
    inp = make_small_input(n_count=4, min_d=1, min_e=1, min_n=1)
    inp = inp.model_copy(update={
        "ward_settings": inp.ward_settings.model_copy(update={"max_consecutive_workdays": 5})
    })
    status, _, cells, _, _ = solve_and_extract(inp)
    assert status in ("OPTIMAL", "FEASIBLE")
    by = {(c.nurse_id, c.date): c.shift for c in cells}
    nurse_ids = {n.id for n in inp.nurses}
    dates_sorted = sorted({c.date for c in cells})
    for nid in nurse_ids:
        streak = 0
        for d in dates_sorted:
            if by.get((nid, d)) in WORK_SHIFTS:
                streak += 1
                assert streak <= 5, f"H-03: nurse {nid} workdays {streak}"
            else:
                streak = 0


# ============================================================
# H-04: 시프트별 전체 최소 인원
# ============================================================


def test_h04_shift_min(simple_input):
    s = simple_input.ward_settings
    status, _, cells, _, _ = solve_and_extract(simple_input)
    assert status in ("OPTIMAL", "FEASIBLE")
    by_date = {}
    for c in cells:
        by_date.setdefault(c.date, []).append(c.shift)
    for d, shifts in by_date.items():
        d_cnt = sum(1 for s2 in shifts if s2 in ("D", "DE"))
        e_cnt = sum(1 for s2 in shifts if s2 in ("E", "DE"))
        n_cnt = sum(1 for s2 in shifts if s2 == "N")
        assert d_cnt >= s.min_d, f"date={d} D={d_cnt} < {s.min_d}"
        assert e_cnt >= s.min_e, f"date={d} E={e_cnt} < {s.min_e}"
        assert n_cnt >= s.min_n, f"date={d} N={n_cnt} < {s.min_n}"


# ============================================================
# H-05: unavailable
# ============================================================


def test_h05_unavailable():
    inp = make_small_input(
        n_count=4, min_d=1, min_e=1, min_n=1,
        wishes=[WishIn(nurse_id=1, date=date(2026, 6, 5), type="unavailable")],
    )
    status, _, cells, _, _ = solve_and_extract(inp)
    assert status in ("OPTIMAL", "FEASIBLE")
    by = {(c.nurse_id, c.date): c.shift for c in cells}
    assert by[(1, date(2026, 6, 5))] == "O", "H-05: unavailable에 work 배정"


# ============================================================
# H-06: N 후 휴식 1일
# ============================================================


def test_h06_rest_after_n(simple_input):
    status, _, cells, _, _ = solve_and_extract(simple_input)
    assert status in ("OPTIMAL", "FEASIBLE")
    by = {(c.nurse_id, c.date): c.shift for c in cells}
    nurse_ids = {n.id for n in simple_input.nurses}
    for nid in nurse_ids:
        dates_sorted = sorted({c.date for c in cells if c.nurse_id == nid})
        for i, d in enumerate(dates_sorted[:-1]):
            if by.get((nid, d)) == "N":
                assert by.get((nid, dates_sorted[i + 1])) == "O", \
                    f"H-06: nurse {nid} N({d}) next != O"


# ============================================================
# H-10: 이전 달 마지막 7일 반영
# ============================================================


def test_h10_month_boundary():
    """이전 달 31일이 N이면 1일은 D가 될 수 없음 (H-01 + H-10)."""
    inp = make_small_input(
        n_count=4, min_d=1, min_e=1, min_n=1,
        prev_cells=[PrevCellIn(nurse_id=1, date=date(2026, 5, 31), shift="N")],
    )
    status, _, cells, _, applied = solve_and_extract(inp)
    assert status in ("OPTIMAL", "FEASIBLE")
    assert "H-10" in applied
    by = {(c.nurse_id, c.date): c.shift for c in cells}
    assert by.get((1, date(2026, 6, 1))) != "D"
    # H-06: prev N → 6/1 O 강제
    assert by.get((1, date(2026, 6, 1))) == "O"


# ============================================================
# H-11: fixed_shift_pattern
# ============================================================


def test_h11_fixed_pattern():
    # 5명: 1=D_ONLY, 나머지는 자유. H-12 미적용 (level min=0).
    inp = make_small_input(
        n_count=5, min_d=1, min_e=1, min_n=1,
        nurses_overrides={1: {"fixed_pattern": "D_ONLY", "level": "L1"}},
    )
    status, _, cells, _, _ = solve_and_extract(inp)
    assert status in ("OPTIMAL", "FEASIBLE")
    by = {(c.nurse_id, c.date): c.shift for c in cells}
    for d in sorted({c.date for c in cells if c.nurse_id == 1}):
        assert by[(1, d)] in ("D", "O"), f"H-11 D_ONLY violated at {d} -> {by[(1, d)]}"


# ============================================================
# H-12: 등급별 시프트당 최소
# ============================================================


def test_h12_level_per_shift():
    """L3 등급별 min_d/min_e/min_n=1 보장. L3 충분히 두기(5명)."""
    levels = [
        ExperienceLevelIn(code="L1", min_d=0, min_e=0, min_n=0),
        ExperienceLevelIn(code="L3", min_d=1, min_e=1, min_n=1),
    ]
    # nurse 1~5를 L3로
    nurses_overrides = {i: {"level": "L3"} for i in range(1, 6)}
    inp = make_small_input(n_count=8, min_d=2, min_e=1, min_n=1,
                           level_overrides=levels, nurses_overrides=nurses_overrides)
    status, _, cells, _, _ = solve_and_extract(inp)
    assert status in ("OPTIMAL", "FEASIBLE")
    by = {(c.nurse_id, c.date): c.shift for c in cells}
    l3_ids = set(range(1, 6))
    dates_sorted = sorted({c.date for c in cells})
    for d in dates_sorted:
        d_l3 = sum(1 for nid in l3_ids if by.get((nid, d)) in ("D", "DE"))
        e_l3 = sum(1 for nid in l3_ids if by.get((nid, d)) in ("E", "DE"))
        n_l3 = sum(1 for nid in l3_ids if by.get((nid, d)) == "N")
        assert d_l3 >= 1, f"H-12 L3 D < 1 at {d}"
        assert e_l3 >= 1, f"H-12 L3 E < 1 at {d}"
        assert n_l3 >= 1, f"H-12 L3 N < 1 at {d}"


# ============================================================
# H-13: 자동 생성 결과에 DE 0건
# ============================================================


def test_h13_no_de_in_auto(simple_input):
    status, _, cells, _, applied = solve_and_extract(simple_input)
    assert status in ("OPTIMAL", "FEASIBLE")
    assert "H-13" in applied
    assert not any(c.shift == "DE" for c in cells), "H-13: DE auto X"


# ============================================================
# H-14: DE → D 금지 (validate 측에서만 의미 — auto에선 DE 자체가 없음)
# ============================================================


def test_h14_de_then_no_d_validator():
    inp = make_small_input(n_count=3, min_d=1, min_e=0, min_n=0)
    cells = []
    for nid in (1, 2, 3):
        for d in range(1, 31):
            cells.append(CellOut(nurse_id=nid, date=date(2026, 6, d), shift="O"))
    # 1번에 DE → D 패턴 삽입
    cells = [c for c in cells if not (c.nurse_id == 1 and c.date in (date(2026, 6, 1), date(2026, 6, 2)))]
    cells.append(CellOut(nurse_id=1, date=date(2026, 6, 1), shift="DE"))
    cells.append(CellOut(nurse_id=1, date=date(2026, 6, 2), shift="D"))
    vinp = ValidateInput(
        year_month="2026-06",
        nurses=inp.nurses,
        wishes=[],
        previous_month_last_week_cells=[],
        experience_levels=inp.experience_levels,
        ward_settings=inp.ward_settings,
        all_cells=cells,
    )
    vs = validate(vinp)
    assert any(v.rule_id == "H-14" for v in vs)
