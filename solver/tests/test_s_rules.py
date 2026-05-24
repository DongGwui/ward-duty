"""L0 — Soft rule tests.

soft 룰은 가중치 0 → 비활성 / 가중치 > 0 → 페널티 최소화 효과.
완전 검증은 어렵지만, applied_rules 마커 + 직관적 경향 검증.
"""

from __future__ import annotations

from datetime import date

from app.model import solve_and_extract
from app.schemas import WishIn
from tests.conftest import make_small_input


def test_s01_balance_off_marker():
    inp = make_small_input(n_count=4, min_d=1, min_e=1, min_n=1)
    inp = inp.model_copy(update={
        "ward_settings": inp.ward_settings.model_copy(update={"weight_balance_off": 10})
    })
    status, _, cells, _, applied = solve_and_extract(inp)
    assert status in ("OPTIMAL", "FEASIBLE")
    assert "S-01" in applied


def test_s02_respect_wishes_marker():
    inp = make_small_input(
        n_count=4, min_d=1, min_e=1, min_n=1,
        wishes=[WishIn(nurse_id=1, date=date(2026, 6, 3), type="off")],
    )
    inp = inp.model_copy(update={
        "ward_settings": inp.ward_settings.model_copy(update={"weight_respect_wishes": 100})
    })
    status, _, cells, _, applied = solve_and_extract(inp)
    assert status in ("OPTIMAL", "FEASIBLE")
    assert "S-02" in applied
    # 강한 가중치라면 보통 충족됨
    by = {(c.nurse_id, c.date): c.shift for c in cells}
    assert by.get((1, date(2026, 6, 3))) == "O"


def test_s03_weekend_balance_marker():
    inp = make_small_input(n_count=4, min_d=1, min_e=1, min_n=1)
    inp = inp.model_copy(update={
        "ward_settings": inp.ward_settings.model_copy(update={"weight_weekend_balance": 5})
    })
    status, _, _, _, applied = solve_and_extract(inp)
    assert status in ("OPTIMAL", "FEASIBLE")
    assert "S-03" in applied


def test_s04_same_shift_streak_marker():
    inp = make_small_input(n_count=4, min_d=1, min_e=1, min_n=1)
    inp = inp.model_copy(update={
        "ward_settings": inp.ward_settings.model_copy(update={"weight_same_shift_streak": 3})
    })
    status, _, _, _, applied = solve_and_extract(inp)
    assert status in ("OPTIMAL", "FEASIBLE")
    assert "S-04" in applied


def test_s05_short_rest_marker():
    inp = make_small_input(n_count=4, min_d=1, min_e=1, min_n=1)
    inp = inp.model_copy(update={
        "ward_settings": inp.ward_settings.model_copy(update={"weight_short_rest_pattern": 4})
    })
    status, _, _, _, applied = solve_and_extract(inp)
    assert status in ("OPTIMAL", "FEASIBLE")
    assert "S-05" in applied
