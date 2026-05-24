"""§10 Infeasibility Policy — 자동 완화 X, 사유 + suggestion 반환."""

from __future__ import annotations

from .schemas import GenerateInput


def suggest_from_input(inp: GenerateInput, applied_rules: list[str]) -> tuple[list[str], str]:
    """infeasible 시 위반 가능성이 높은 룰과 권장 조치 텍스트 추론.

    실제 CP-SAT unsat-core는 별도 API (assumptions) 도입 후 v2에서 정밀화.
    여기서는 입력 통계 기반 휴리스틱.
    """
    suggestions: list[str] = []
    violated: list[str] = []

    n_count = len(inp.nurses)
    s = inp.ward_settings
    min_total = s.min_d + s.min_e + s.min_n
    if n_count < min_total:
        violated.append("H-04")
        suggestions.append(
            f"전체 인원({n_count}명) < 시프트당 최소 합({min_total}명). "
            f"min_d/min_e/min_n을 낮추거나 인원을 늘리세요."
        )

    # 등급별 min 합과 보유 인원 비교
    from collections import Counter

    level_cnt = Counter(n.level for n in inp.nurses)
    for lv in inp.experience_levels:
        held = level_cnt.get(lv.code, 0)
        need_max = max(lv.min_d, lv.min_e, lv.min_n)
        if held < need_max:
            violated.append("H-12")
            suggestions.append(
                f"등급 {lv.code} 보유 {held}명 < 시프트당 필요 {need_max}명. "
                f"{lv.code}.min_d/min_e/min_n을 낮추거나 해당 등급 인원을 추가하세요."
            )

    # unavailable 비율
    unavail = sum(1 for w in inp.wishes if w.type == "unavailable")
    if unavail > n_count * len(_days_in_month(inp.year_month)) * 0.3:
        violated.append("H-05")
        suggestions.append(
            f"unavailable 희망({unavail}건)이 과도합니다. 일부 희망을 'off' soft로 전환 검토."
        )

    # nk 수
    nk = sum(1 for n in inp.nurses if n.is_night_keeper)
    if nk > 0 and nk * 30 < s.min_n * len(_days_in_month(inp.year_month)):
        # nk가 너무 적은 경우는 문제 안 됨. 너무 많으면? 일단 패스
        pass

    # 고정 패턴 비율
    fixed = sum(1 for n in inp.nurses if n.fixed_pattern)
    if fixed > n_count * 0.5:
        violated.append("H-11")
        suggestions.append(
            f"고정 패턴 인원({fixed}/{n_count})이 과반입니다. 일반 로테이션 가능 인원을 늘리세요."
        )

    if not violated:
        suggestions.append("입력 통계상 명백한 위반 후보 없음. ward_settings 가중치·hard 임계값 검토 필요.")

    return violated, " | ".join(suggestions)


def _days_in_month(ym: str) -> list[int]:
    import calendar

    y, m = map(int, ym.split("-"))
    _, d = calendar.monthrange(y, m)
    return list(range(1, d + 1))
