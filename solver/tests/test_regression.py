"""L0 — Regression Test Suite (KPI 검증).

Design Ref: §8.3 L0 Regression
- 5종 fixture로 솔버 회귀.
- "그대로 쓸 수 있는 초안" 비율 = hard 위반 0인 해를 얻은 비율.
- KPI 목표: 4/5 이상 hard-clean feasible (= 80%).

이 테스트가 통과한다면 KPI 80%를 솔버 차원에서 만족.
"""

from __future__ import annotations

import pytest

from app.model import solve_and_extract
from app.schemas import ValidateInput
from app.validator import validate
from tests.conftest import load_fixture

FIXTURES = [
    "simple_30_jun",
    "tight_15_jul",
    "night_keepers_25_aug",
    "fixed_patterns_20_sep",
    # infeasible_oct은 의도적 infeasible — KPI 카운트에서 제외
]


@pytest.mark.parametrize("name", FIXTURES)
def test_regression_solves_and_clean(name: str) -> None:
    """각 fixture가 OPTIMAL/FEASIBLE이고, 결과가 hard 위반 0이어야 함."""
    inp = load_fixture(name)
    status, _, cells, elapsed_ms, applied = solve_and_extract(inp)
    assert status in ("OPTIMAL", "FEASIBLE"), f"{name}: solver status={status}"

    # SC-2: 30명×31일 ≤60s — fixture별 max_time_seconds 내
    assert elapsed_ms <= inp.max_time_seconds * 1000 + 5000, \
        f"{name}: elapsed_ms={elapsed_ms} > {inp.max_time_seconds * 1000}+5s grace"

    # SC-3: hard 위반 0건
    vinp = ValidateInput(
        year_month=inp.year_month,
        nurses=inp.nurses,
        wishes=inp.wishes,
        previous_month_last_week_cells=inp.previous_month_last_week_cells,
        experience_levels=inp.experience_levels,
        ward_settings=inp.ward_settings,
        all_cells=cells,
    )
    violations = validate(vinp)
    hard = [v for v in violations if v.severity == "hard"]
    assert not hard, f"{name}: hard violations {[v.rule_id for v in hard]}"


def test_infeasible_fixture_returns_infeasible() -> None:
    """infeasible_oct은 정확히 INFEASIBLE을 반환하고 violated_rule_ids가 채워져야 함."""
    from app.infeasibility import suggest_from_input

    inp = load_fixture("infeasible_oct")
    status, _, cells, _, applied = solve_and_extract(inp)
    assert status == "INFEASIBLE", f"expected INFEASIBLE, got {status}"
    violated, suggestion = suggest_from_input(inp, applied)
    assert violated, "violated_rule_ids should be populated"
    assert suggestion, "suggestion text required"


def test_kpi_80_percent() -> None:
    """SC-1: 자동 초안 80%+ 가 hard-clean으로 사용 가능.

    5종 중 1종은 의도적 infeasible — 나머지 4/4 = 100% 이상 hard-clean이면 KPI 충족.
    """
    success = 0
    total = len(FIXTURES)  # infeasible 제외
    for name in FIXTURES:
        inp = load_fixture(name)
        status, _, cells, _, _ = solve_and_extract(inp)
        if status not in ("OPTIMAL", "FEASIBLE"):
            continue
        vinp = ValidateInput(
            year_month=inp.year_month,
            nurses=inp.nurses,
            wishes=inp.wishes,
            previous_month_last_week_cells=inp.previous_month_last_week_cells,
            experience_levels=inp.experience_levels,
            ward_settings=inp.ward_settings,
            all_cells=cells,
        )
        hard = [v for v in validate(vinp) if v.severity == "hard"]
        if not hard:
            success += 1
    rate = success / total
    assert rate >= 0.8, f"KPI 미달: {success}/{total} = {rate:.0%} < 80%"
