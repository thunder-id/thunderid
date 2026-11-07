# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

"""HTTP contract tests with bounded app-level doubles."""

import asyncio
from types import SimpleNamespace

import httpx
import pytest

from wayfinder_agent.app import create_app
from wayfinder_agent.auth import AuthError, Principal
from wayfinder_agent.settings import Settings


class Auth:
    closed = False
    completed = None

    async def initialize(self):
        pass

    async def aclose(self):
        self.closed = True

    async def context(self, principal, session_id):
        if session_id == "foreign":
            raise AuthError("ACCESS_DENIED", "This chat belongs to another account.", 403)
        return SimpleNamespace(id=session_id or "session")

    async def finish_consent(self, session_id, **values):
        self.completed = values
        if values["state"] != "valid":
            raise AuthError("INVALID_CONSENT", "The consent response is invalid.", 400)
        return SimpleNamespace(id=session_id)


class Verifier:
    async def verify(self, header):
        if header != "Bearer valid":
            raise AuthError("INVALID_BEARER", "A valid chat access token is required.")
        return Principal("alice")


class Assistant:
    model_ready = True
    failure = False

    async def initialize(self):
        pass

    async def aclose(self):
        pass

    async def reply(self, context, message):
        if self.failure:
            raise RuntimeError("secret-from-provider")
        return {"type": "response", "message": "Hello", "session_id": context.id}


class Upgrades:
    enabled = False

    def __init__(self):
        self.started = asyncio.Event()
        self.finished = asyncio.Event()

    async def initialize(self):
        pass

    async def aclose(self):
        pass

    async def process_one(self, principal):
        assert principal.subject == "alice"
        self.started.set()
        await self.finished.wait()
        return {"status": "processed"}


@pytest.fixture
def components():
    settings = Settings(
        thunderid_base_url="https://issuer.example",
        agent_id="client",
        agent_secret="secret",
        mcp_server_url="http://booking.example/mcp",
        agent_redirect_uri="http://travel.example/agent-callback",
        chat_resource_url="http://concierge.example/chat",
        frontend_origin="http://travel.example",
    )
    auth, assistant, upgrades = Auth(), Assistant(), Upgrades()
    app = create_app(
        settings, auth=auth, verifier=Verifier(), concierge=assistant, upgrades=upgrades
    )
    return app, auth, assistant, upgrades


async def test_chat_contract_authentication_validation_and_ownership(components):
    app, auth, _, _ = components
    async with (
        app.router.lifespan_context(app),
        httpx.AsyncClient(
            transport=httpx.ASGITransport(app=app), base_url="http://sample"
        ) as client,
    ):
        assert (await client.get("/health")).json()["model_configured"] is True
        assert (await client.get("/chat/access")).status_code == 401
        headers = {"Authorization": "Bearer valid", "Origin": "http://travel.example"}
        assert (await client.get("/chat/access", headers=headers)).json() == {"authorized": True}
        response = await client.post("/chat", headers=headers, json={"message": "Hello"})
        assert response.json() == {
            "type": "response",
            "message": "Hello",
            "session_id": "session",
        }
        assert response.headers["access-control-allow-origin"] == "http://travel.example"
        assert (
            await client.post("/chat", headers=headers, json={"message": ""})
        ).status_code == 400
        assert (await client.post("/chat", headers=headers, content="x" * 65537)).status_code == 413
        assert (await client.post("/chat", headers=headers, content="[" * 5000)).status_code == 400
        assert (
            await client.post(
                "/chat", headers=headers, json={"message": "Hello", "session_id": "foreign"}
            )
        ).status_code == 403
        assert (
            await client.post(
                "/chat",
                headers={**headers, "Origin": "https://untrusted.example"},
                json={"message": "Hello"},
            )
        ).status_code == 403
    assert auth.closed


async def test_consent_keeps_frontend_contract_and_requires_state(components):
    app, auth, _, _ = components
    async with (
        app.router.lifespan_context(app),
        httpx.AsyncClient(
            transport=httpx.ASGITransport(app=app), base_url="http://sample"
        ) as client,
    ):
        response = await client.post(
            "/chat/consent", json={"session_id": "session", "code": "code", "state": "valid"}
        )
        assert response.json() == {"type": "consent_received", "session_id": "session"}
        assert auth.completed["code"] == "code"
        assert (
            await client.post("/chat/consent", json={"session_id": "session", "code": "code"})
        ).status_code == 400
        assert (
            await client.post(
                "/chat/consent",
                json={"session_id": "session", "code": "code", "state": "wrong"},
            )
        ).status_code == 400


async def test_model_failures_are_sanitized(components):
    app, _, assistant, _ = components
    assistant.failure = True
    async with (
        app.router.lifespan_context(app),
        httpx.AsyncClient(
            transport=httpx.ASGITransport(app=app),
            base_url="http://sample",
        ) as client,
    ):
        response = await client.post(
            "/chat",
            headers={"Authorization": "Bearer valid", "Origin": "http://travel.example"},
            json={"message": "Hello"},
        )
        assert response.status_code == 502
        assert response.headers["access-control-allow-origin"] == "http://travel.example"
        assert "secret-from-provider" not in response.text


async def test_upgrade_trigger_is_authenticated_feature_guarded_and_single_flight(components):
    app, _, _, upgrades = components
    async with (
        app.router.lifespan_context(app),
        httpx.AsyncClient(
            transport=httpx.ASGITransport(app=app), base_url="http://sample"
        ) as client,
    ):
        assert (await client.post("/api/demo/process-upgrades")).status_code == 401
        headers = {"Authorization": "Bearer valid"}
        assert (await client.post("/api/demo/process-upgrades", headers=headers)).status_code == 404
        upgrades.enabled = True
        assert (await client.post("/api/demo/process-upgrades", headers=headers)).status_code == 202
        await upgrades.started.wait()
        assert (await client.get("/health")).json()["upgrade_status"] == "running"
        assert (await client.post("/api/demo/process-upgrades", headers=headers)).status_code == 409
        upgrades.finished.set()
        await app.state.upgrade_task
        assert app.state.last_upgrade_status == "processed"
