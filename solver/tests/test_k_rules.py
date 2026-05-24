"""L0 — Night-Keeper rule (K-01)."""

from __future__ import annotations

from app.model import solve_and_extract
from tests.conftest import make_small_input


def test_k01_nk_only_n_or_o():
    """is_night_keeper=True인 간호사는 그 달 N/O만."""
    inp = make_small_input(
        n_count=5,
        min_d=1, min_e=1, min_n=1,
        nurses_overrides={1: {"is_night_keeper": True, "level": "L3"}},
    )
    status, _, cells, _, applied = solve_and_extract(inp)
    assert status in ("OPTIMAL", "FEASIBLE")
    assert "K-01" in applied
    by = {(c.nurse_id, c.date): c.shift for c in cells}
    dates = {c.date for c in cells if c.nurse_id == 1}
    for d in dates:
        assert by[(1, d)] in ("N", "O"), f"K-01: nurse 1 has {by[(1, d)]} at {d}"
