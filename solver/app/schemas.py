"""Pydantic 입출력 스키마. Go side `internal/solver/schemas.go`와 JSON 호환.

Design Ref: §4.3 솔버 API
"""

import datetime as _dt
from datetime import date
from typing import List, Literal, Optional

from pydantic import BaseModel, Field, field_validator

ShiftCode = Literal["D", "E", "N", "O", "DE"]
WishType = Literal["off", "d", "e", "n", "unavailable"]
FixedPattern = Literal["D_ONLY", "E_ONLY", "N_ONLY", "WEEKDAY_D", "WEEKDAY_E"]


# ============================================================
# Input
# ============================================================


class NurseIn(BaseModel):
    id: int
    level: str                                # experience_levels.code (G-04)
    fixed_pattern: Optional[FixedPattern] = None  # H-11
    is_night_keeper: bool = False             # K-01


class WishIn(BaseModel):
    nurse_id: int
    date: date
    type: WishType


class PrevCellIn(BaseModel):
    """이전 달 마지막 lookback_days 일치 cells (H-10). 솔버 상수로 취급."""

    nurse_id: int
    date: date
    shift: ShiftCode


class ExperienceLevelIn(BaseModel):
    code: str
    min_d: int = 0
    min_e: int = 0
    min_n: int = 0
    weight_coverage: int = 1
    weight_d_assignment: int = 0
    weight_e_assignment: int = 0
    weight_n_assignment: int = 0


class WardSettingsIn(BaseModel):
    min_d: int = 3
    min_e: int = 2
    min_n: int = 2
    max_consecutive_n: int = 3
    min_rest_after_n: int = 1
    max_consecutive_workdays: int = 5
    balance_off_tolerance: int = 1
    previous_month_lookback_days: int = 7  # H11 fix: H-10
    weight_balance_off: int = 10
    weight_respect_wishes: int = 8
    weight_weekend_balance: int = 5
    weight_same_shift_streak: int = 3
    weight_short_rest_pattern: int = 4


class GenerateInput(BaseModel):
    """POST /generate 요청. Design §4.3."""

    year_month: str = Field(pattern=r"^\d{4}-\d{2}$")
    nurses: List[NurseIn]
    wishes: List[WishIn] = []
    previous_month_last_week_cells: List[PrevCellIn] = []  # H-10
    experience_levels: List[ExperienceLevelIn]
    ward_settings: WardSettingsIn = WardSettingsIn()
    max_time_seconds: int = 55

    @field_validator("nurses")
    @classmethod
    def _nurses_unique(cls, v):
        ids = [n.id for n in v]
        if len(set(ids)) != len(ids):
            raise ValueError("nurses에 중복 id가 있습니다")
        return v


class ValidateInput(BaseModel):
    """POST /validate 요청. 수동 수정 검증용."""

    year_month: str = Field(pattern=r"^\d{4}-\d{2}$")
    nurses: List[NurseIn]
    wishes: List[WishIn] = []
    previous_month_last_week_cells: List[PrevCellIn] = []
    experience_levels: List[ExperienceLevelIn]
    ward_settings: WardSettingsIn = WardSettingsIn()
    all_cells: List["CellOut"]


# ============================================================
# Output
# ============================================================


class CellOut(BaseModel):
    nurse_id: int
    date: date
    shift: ShiftCode


class GenerateOutput(BaseModel):
    status: Literal["ok", "infeasible", "timeout", "error"]
    solver_status: str
    objective_value: Optional[int] = None
    cells: List[CellOut] = []
    applied_rules: List[str] = []
    elapsed_ms: int
    violated_rule_ids: List[str] = []
    suggestion: Optional[str] = None


class Violation(BaseModel):
    rule_id: str
    severity: Literal["hard", "soft"]
    message: str
    cell_ids: List[int] = []
    nurse_id: Optional[int] = None
    # field 이름 'date'와 datetime.date의 self-shadow 회피를 위해 module 경로 사용
    date: Optional[_dt.date] = None


class ValidateOutput(BaseModel):
    violations: List[Violation]
    hard_count: int
    soft_count: int


ValidateInput.model_rebuild()
