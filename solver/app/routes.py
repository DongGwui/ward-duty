"""FastAPI routes — Design Ref §4.3."""

from __future__ import annotations

import logging
import os
import secrets
import time

from fastapi import APIRouter, Header, HTTPException, status

from . import __version__
from .infeasibility import suggest_from_input
from .model import solve_and_extract
from .schemas import (
    GenerateInput,
    GenerateOutput,
    ValidateInput,
    ValidateOutput,
)
from .validator import validate as run_validate

logger = logging.getLogger(__name__)
router = APIRouter()

_EXPECTED_TOKEN = os.environ.get("SOLVER_INTERNAL_TOKEN", "")
_ENV = os.environ.get("ENV", "development").lower()


def _ensure_token_configured() -> None:
    """C4 fix: 운영(production)에서 토큰 미설정 시 startup 차단.

    main.py가 모듈 import 시 본 함수를 호출한다.
    test/dev에서는 명시적으로 ENV=development 권장 + 토큰 미설정 허용.
    """
    if _ENV == "production" and not _EXPECTED_TOKEN:
        raise RuntimeError(
            "SOLVER_INTERNAL_TOKEN is required in production (ENV=production)"
        )


def _verify_token(x_internal_token: str | None) -> None:
    """Design §7: solver internal-only.

    C4 fix:
      - constant-time 비교 (timing 공격 차단)
      - dev에서 토큰 미설정이면 통과, production에서는 startup 단계에서 이미 차단됨
    """
    if not _EXPECTED_TOKEN:
        if _ENV == "production":
            # 방어적 — startup 가드를 우회한 경우
            raise HTTPException(
                status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
                detail="solver token misconfigured",
            )
        return
    provided = x_internal_token or ""
    if not secrets.compare_digest(provided, _EXPECTED_TOKEN):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="invalid internal token",
        )


@router.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok", "version": __version__}


@router.post("/generate", response_model=GenerateOutput)
def generate(inp: GenerateInput, x_internal_token: str | None = Header(default=None)) -> GenerateOutput:
    _verify_token(x_internal_token)
    t0 = time.perf_counter()
    try:
        status_name, obj, cells, elapsed_ms, applied = solve_and_extract(inp)
    except Exception as e:  # noqa: BLE001
        logger.exception("solver error")
        return GenerateOutput(
            status="error",
            solver_status=str(e)[:200],
            elapsed_ms=int((time.perf_counter() - t0) * 1000),
        )

    if status_name in ("OPTIMAL", "FEASIBLE"):
        return GenerateOutput(
            status="ok",
            solver_status=status_name,
            objective_value=obj,
            cells=cells,
            applied_rules=applied,
            elapsed_ms=elapsed_ms,
        )

    if status_name == "INFEASIBLE":
        violated, suggestion = suggest_from_input(inp, applied)
        return GenerateOutput(
            status="infeasible",
            solver_status=status_name,
            elapsed_ms=elapsed_ms,
            applied_rules=applied,
            violated_rule_ids=violated,
            suggestion=suggestion,
        )

    if status_name in ("UNKNOWN", "MODEL_INVALID"):
        return GenerateOutput(
            status="timeout" if status_name == "UNKNOWN" else "error",
            solver_status=status_name,
            elapsed_ms=elapsed_ms,
            applied_rules=applied,
        )

    return GenerateOutput(
        status="error",
        solver_status=status_name,
        elapsed_ms=elapsed_ms,
        applied_rules=applied,
    )


@router.post("/validate", response_model=ValidateOutput)
def validate_endpoint(
    inp: ValidateInput, x_internal_token: str | None = Header(default=None)
) -> ValidateOutput:
    _verify_token(x_internal_token)
    violations = run_validate(inp)
    return ValidateOutput(
        violations=violations,
        hard_count=sum(1 for v in violations if v.severity == "hard"),
        soft_count=sum(1 for v in violations if v.severity == "soft"),
    )
