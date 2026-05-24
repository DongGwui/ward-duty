"""FastAPI 진입점."""

from __future__ import annotations

import logging
import os

from fastapi import FastAPI

from .routes import _ensure_token_configured, router

logging.basicConfig(
    level=os.environ.get("LOG_LEVEL", "INFO").upper(),
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)

# C4 fix: production에서 SOLVER_INTERNAL_TOKEN 미설정이면 즉시 fail.
_ensure_token_configured()

app = FastAPI(title="ward-duty-solver", version="0.1.0")
app.include_router(router)
