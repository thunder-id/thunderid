# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

"""HTTP contract consumed by the existing Wayfinder frontend."""

from __future__ import annotations

import asyncio
import json
from contextlib import asynccontextmanager, suppress
from typing import Any

from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse
from thunderid import ThunderIDError

from .auth import AuthError, BearerVerifier, ConciergeAuth
from .concierge import Concierge
from .settings import Settings
from .upgrades import UpgradeProcessor


async def _body(request: Request) -> dict[str, Any]:
    content = bytearray()
    async for chunk in request.stream():
        if len(content) + len(chunk) > 65536:
            raise AuthError("INVALID_REQUEST", "The request is too large.", 413)
        content.extend(chunk)
    try:
        value = json.loads(content)
    except (ValueError, UnicodeError, RecursionError):
        raise AuthError("INVALID_REQUEST", "A JSON object is required.", 400) from None
    if not isinstance(value, dict):
        raise AuthError("INVALID_REQUEST", "A JSON object is required.", 400)
    return value


def _text(
    body: dict[str, Any], key: str, *, required: bool = False, maximum: int = 8192
) -> str | None:
    value = body.get(key)
    if value is None and not required:
        return None
    if not isinstance(value, str) or not value.strip() or len(value) > maximum:
        raise AuthError("INVALID_REQUEST", "The request fields are invalid.", 400)
    return value


def create_app(
    settings: Settings, *, auth=None, verifier=None, concierge=None, upgrades=None
) -> FastAPI:
    authentication = auth if auth is not None else ConciergeAuth(settings)
    bearer = verifier if verifier is not None else BearerVerifier(settings)
    assistant = concierge if concierge is not None else Concierge(settings, authentication)
    processor = upgrades if upgrades is not None else UpgradeProcessor(settings)

    @asynccontextmanager
    async def lifespan(app: FastAPI):
        app.state.upgrade_task = None
        app.state.last_upgrade_status = None
        try:
            await authentication.initialize()
            await assistant.initialize()
            await processor.initialize()
            yield
        finally:
            task = app.state.upgrade_task
            if task is not None and not task.done():
                task.cancel()
                with suppress(asyncio.CancelledError):
                    await task
            try:
                await processor.aclose()
            finally:
                try:
                    await assistant.aclose()
                finally:
                    await authentication.aclose()

    app = FastAPI(lifespan=lifespan, docs_url=None, redoc_url=None)

    @app.middleware("http")
    async def check_origin(request: Request, call_next):
        origin = request.headers.get("origin")
        if origin is not None and origin != settings.frontend_origin:
            return JSONResponse({"error": "This origin is not allowed."}, status_code=403)
        try:
            return await call_next(request)
        except Exception:  # noqa: BLE001 - provider failures must not escape into server traceback logs.
            return JSONResponse(
                {"error": "The Concierge could not complete the request."}, status_code=502
            )

    # Added last so it wraps check_origin and its sanitized 502 responses keep CORS headers.
    app.add_middleware(
        CORSMiddleware,
        allow_origins=[settings.frontend_origin],
        allow_methods=["GET", "POST"],
        allow_headers=["Authorization", "Content-Type"],
    )

    @app.exception_handler(AuthError)
    async def auth_error(request: Request, error: AuthError):
        payload = {"error": str(error), "code": error.code}
        if request.url.path == "/chat/access":
            payload["authorized"] = False
        return JSONResponse(payload, status_code=error.status_code)

    @app.exception_handler(ThunderIDError)
    async def sdk_error(request: Request, error: ThunderIDError):
        return JSONResponse(
            {"error": "The identity operation could not complete.", "code": error.code},
            status_code=502,
        )

    @app.get("/health")
    async def health():
        return {
            "status": "ok",
            "model_configured": assistant.model_ready,
            "upgrades_enabled": processor.enabled,
            "upgrade_status": app.state.last_upgrade_status or "idle",
        }

    @app.get("/chat/access")
    async def access(request: Request):
        await bearer.verify(request.headers.get("authorization"))
        return {"authorized": True}

    @app.post("/chat")
    async def chat(request: Request):
        principal = await bearer.verify(request.headers.get("authorization"))
        body = await _body(request)
        message = _text(body, "message", required=True)
        context = await authentication.context(principal, _text(body, "session_id", maximum=128))
        return await assistant.reply(context, message)

    @app.post("/chat/consent")
    async def consent(request: Request):
        body = await _body(request)
        # The frontend sends no bearer here. SDK state validation and the original subject bind this response.
        context = await authentication.finish_consent(
            _text(body, "session_id", required=True, maximum=128),
            code=_text(body, "code"),
            state=_text(body, "state", required=True, maximum=512),
            request_id=_text(body, "request_id", maximum=128),
            error=_text(body, "error", maximum=128),
        )
        return {"type": "consent_received", "session_id": context.id}

    async def process(principal):
        try:
            result = await processor.process_one(principal)
            app.state.last_upgrade_status = result["status"]
        except asyncio.CancelledError:
            app.state.last_upgrade_status = "canceled"
            raise
        except Exception:  # noqa: BLE001 - background failures expose a static status only.
            app.state.last_upgrade_status = "failed"

    @app.post("/api/demo/process-upgrades", status_code=202)
    async def process_upgrades(request: Request):
        principal = await bearer.verify(request.headers.get("authorization"))
        if not processor.enabled:
            raise AuthError("UPGRADES_DISABLED", "The CIBA upgrade processor is disabled.", 404)
        task = app.state.upgrade_task
        if task is not None and not task.done():
            raise AuthError(
                "UPGRADE_IN_PROGRESS", "An upgrade approval is already in progress.", 409
            )
        app.state.last_upgrade_status = "running"
        app.state.upgrade_task = asyncio.create_task(process(principal))
        return {
            "message": "Upgrade processing started. Approve the request through the configured channel."
        }

    return app


def main() -> None:
    import uvicorn

    settings = Settings.from_env()
    uvicorn.run(create_app(settings), host=settings.host, port=settings.port, access_log=False)


if __name__ == "__main__":
    main()
