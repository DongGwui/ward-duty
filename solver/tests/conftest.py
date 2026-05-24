"""pytest 공통 fixtures."""

from __future__ import annotations

import json
from datetime import date
from pathlib import Path

import pytest

from app.schemas import (
    ExperienceLevelIn,
    GenerateInput,
    NurseIn,
    WardSettingsIn,
)

FIXTURES_DIR = Path(__file__).parent / "fixtures"


def load_fixture(name: str) -> GenerateInput:
    path = FIXTURES_DIR / f"{name}.json"
    return GenerateInput.model_validate_json(path.read_text())


@pytest.fixture
def simple_input() -> GenerateInput:
    """단순 4명 / 30일 입력 — 단위 테스트 빠른 검증용.
    H-12 비활성 (등급별 min=0)."""
    levels = [
        ExperienceLevelIn(code="L1", min_d=0, min_e=0, min_n=0),
        ExperienceLevelIn(code="L3", min_d=0, min_e=0, min_n=0),
    ]
    nurses = [
        NurseIn(id=1, level="L3"),
        NurseIn(id=2, level="L3"),
        NurseIn(id=3, level="L1"),
        NurseIn(id=4, level="L1"),
    ]
    settings = WardSettingsIn(min_d=1, min_e=1, min_n=1, max_consecutive_n=2,
                              min_rest_after_n=1, max_consecutive_workdays=5,
                              weight_balance_off=0,
                              weight_respect_wishes=0,
                              weight_weekend_balance=0,
                              weight_same_shift_streak=0,
                              weight_short_rest_pattern=0)
    return GenerateInput(
        year_month="2026-06",
        nurses=nurses,
        wishes=[],
        previous_month_last_week_cells=[],
        experience_levels=levels,
        ward_settings=settings,
        max_time_seconds=10,
    )


def make_small_input(
    *,
    n_count: int = 6,
    year_month: str = "2026-06",
    min_d: int = 2,
    min_e: int = 1,
    min_n: int = 1,
    nurses_overrides=None,
    wishes=None,
    prev_cells=None,
    level_overrides=None,
) -> GenerateInput:
    """파라미터화된 작은 입력 생성기.

    기본 levels는 모두 min_*=0 → 각 테스트가 자기 룰만 검증.
    H-12 테스트는 level_overrides로 명시.
    """
    levels = level_overrides or [
        ExperienceLevelIn(code="L1", min_d=0, min_e=0, min_n=0),
        ExperienceLevelIn(code="L3", min_d=0, min_e=0, min_n=0),
    ]
    nurses = []
    for i in range(1, n_count + 1):
        overrides = (nurses_overrides or {}).get(i, {})
        level = overrides.get("level", "L3" if i <= 2 else "L1")
        nurses.append(NurseIn(id=i, level=level,
                              fixed_pattern=overrides.get("fixed_pattern"),
                              is_night_keeper=overrides.get("is_night_keeper", False)))
    settings = WardSettingsIn(
        min_d=min_d, min_e=min_e, min_n=min_n,
        max_consecutive_n=2, min_rest_after_n=1, max_consecutive_workdays=5,
        weight_balance_off=0, weight_respect_wishes=0,
        weight_weekend_balance=0, weight_same_shift_streak=0,
        weight_short_rest_pattern=0,
    )
    return GenerateInput(
        year_month=year_month,
        nurses=nurses,
        wishes=[w for w in (wishes or [])],
        previous_month_last_week_cells=[c for c in (prev_cells or [])],
        experience_levels=levels,
        ward_settings=settings,
        max_time_seconds=10,
    )
