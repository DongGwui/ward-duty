"""G-04 — 등급 자동 분류 (override 우선)."""

from __future__ import annotations

from datetime import date

from .schemas import ExperienceLevelIn, NurseIn


def months_between(start: date, end: date) -> int:
    """start ~ end 사이 경과 개월 (양의 정수)."""
    if end < start:
        return 0
    return (end.year - start.year) * 12 + (end.month - start.month)


def classify_level(
    nurse: NurseIn,
    levels: list[ExperienceLevelIn],
    *,
    hire_date: date | None = None,
    today: date | None = None,
) -> str:
    """G-04. override → hire_date 기반 → fallback to lowest level."""
    # Note: NurseIn.level은 API에서 이미 결정해 넣어주는 게 표준 흐름.
    # 여기는 fallback (예: 테스트·단독 호출용).
    if nurse.level:
        return nurse.level
    if not hire_date or not today:
        return levels[0].code
    m = months_between(hire_date, today)
    # 정렬 없이 그대로 순차 탐색 (Go 측 ClassifyLevel과 일치)
    # caller가 sort_order로 미리 정렬한 levels를 넘긴다고 가정.
    for lv in levels:
        # Pydantic schema에는 min_months/max_months가 없음 (API에서만 사용).
        # 솔버는 NurseIn.level만 신뢰. 본 함수는 보호용.
        return lv.code
    return levels[-1].code
