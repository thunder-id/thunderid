# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

"""Focused CIBA orchestration tests; protocol details remain SDK-owned."""

import asyncio
import json
from dataclasses import replace

import pytest
from langchain_mcp_adapters.interceptors import MCPToolCallRequest
from thunderid import ThunderIDRuntimeError, TokenResponse

from wayfinder_agent import upgrades as module
from wayfinder_agent.auth import AuthError, Principal
from wayfinder_agent.settings import Settings
from wayfinder_agent.upgrades import UpgradeProcessor


@pytest.fixture
def settings():
    return Settings(
        thunderid_base_url="https://issuer.example",
        agent_id="client",
        agent_secret="secret",
        mcp_server_url="http://booking.example/mcp",
        agent_redirect_uri="http://travel.example/agent-callback",
        chat_resource_url="http://concierge.example/chat",
        frontend_origin="http://travel.example",
        upgrade_agent_id="upgrade",
        upgrade_agent_secret="upgrade-secret",
        upgrade_scheduler_enabled=True,
    )


class Client:
    def __init__(self, config):
        self.config = config
        self.machine_requests = []
        self.initiation = None
        self.request = object()
        self.user_tokens = TokenResponse("user")
        self.cleaned = 0
        self.closed = False
        self.error = None
        self.waiting = None

    async def initialize(self):
        pass

    async def get_agent_token(self, **values):
        self.machine_requests.append(values)
        return TokenResponse("machine")

    async def initiate_backchannel_authentication(self, **values):
        self.initiation = values
        return self.request

    async def await_backchannel_authentication(self, request, timeout):
        assert request is self.request and timeout == 120
        if self.error:
            raise self.error
        if self.waiting:
            self.waiting.set()
            await asyncio.Event().wait()
        return self.user_tokens

    async def get_obo_token(self, *, authorization):
        assert authorization is self.user_tokens
        return self.user_tokens

    async def sign_out(self):
        self.cleaned += 1

    async def aclose(self):
        self.closed = True


def tool_client(monkeypatch, pending, process_result=None):
    received = []

    class Tool:
        def __init__(self, name, interceptor):
            self.name, self.interceptor = name, interceptor

        async def ainvoke(self, arguments):
            async def handler(request):
                received.append(request)
                value = (
                    {"request": pending, "pendingCount": int(pending is not None)}
                    if self.name == "get_pending_upgrade"
                    else ({"status": "success"} if process_result is None else process_result)
                )
                return [{"type": "text", "text": json.dumps(value)}]

            return await self.interceptor(
                MCPToolCallRequest(name=self.name, args=arguments, server_name="travel"), handler
            )

    class MCP:
        def __init__(self, interceptor):
            self.interceptor = interceptor

        async def get_tools(self):
            return [
                Tool("get_pending_upgrade", self.interceptor),
                Tool("process_upgrade", self.interceptor),
            ]

    monkeypatch.setattr(
        module, "mcp_client", lambda settings, own, interceptors: MCP(interceptors[0])
    )
    monkeypatch.setattr(module, "AsyncThunderIDClient", Client)
    return received


async def test_approved_owner_uses_separate_sdk_client_and_fresh_token_per_tool(
    settings, monkeypatch
):
    received = tool_client(
        monkeypatch, {"id": "upgrade-one", "userId": "alice", "email": "alice@example.test"}
    )
    processor = UpgradeProcessor(settings)
    await processor.initialize()
    result = await processor.process_one(Principal("alice"))
    assert result == {"status": "processed"}
    assert processor._client.config.client_id == "upgrade"
    assert processor._client.machine_requests == [
        {"resource": settings.mcp_server_url, "scopes": ["upgrade:read", "upgrade:search"]}
    ]
    assert processor._client.initiation["login_hint"] == "alice@example.test"
    assert processor._client.initiation["resource"] == settings.mcp_server_url
    assert processor._client.initiation["scopes"] == ["openid", "upgrade:process"]
    assert [request.headers["Authorization"] for request in received] == [
        "Bearer machine",
        "Bearer user",
    ]
    assert received[1].args == {"upgradeRequestId": "upgrade-one"}
    assert processor._client.cleaned == 1
    await processor.aclose()
    assert processor._client.closed


async def test_other_account_cannot_trigger_someone_elses_approval(settings, monkeypatch):
    received = tool_client(
        monkeypatch, {"id": "upgrade-one", "userId": "bob", "email": "bob@example.test"}
    )
    processor = UpgradeProcessor(settings)
    await processor.initialize()
    with pytest.raises(AuthError) as error:
        await processor.process_one(Principal("alice"))
    assert error.value.status_code == 403
    assert processor._client.initiation is None
    assert len(received) == 1
    assert processor._client.cleaned == 1


async def test_denial_does_not_process_upgrade_and_clears_local_authorization(
    settings, monkeypatch
):
    received = tool_client(
        monkeypatch, {"id": "upgrade-one", "userId": "alice", "email": "alice@example.test"}
    )
    processor = UpgradeProcessor(settings)
    await processor.initialize()
    processor._client.error = ThunderIDRuntimeError("ACCESS_DENIED", "Approval denied.")
    with pytest.raises(ThunderIDRuntimeError):
        await processor.process_one(Principal("alice"))
    assert len(received) == 1
    assert processor._client.cleaned == 1


async def test_processor_cancellation_cleans_up_sdk_context(settings, monkeypatch):
    received = tool_client(
        monkeypatch, {"id": "upgrade-one", "userId": "alice", "email": "alice@example.test"}
    )
    processor = UpgradeProcessor(settings)
    await processor.initialize()
    processor._client.waiting = asyncio.Event()
    task = asyncio.create_task(processor.process_one(Principal("alice")))
    await processor._client.waiting.wait()
    task.cancel()
    with pytest.raises(asyncio.CancelledError):
        await task
    assert processor._client.cleaned == 1
    assert len(received) == 1


async def test_disabled_processor_does_not_construct_an_oauth_client(settings, monkeypatch):
    monkeypatch.setattr(
        module,
        "AsyncThunderIDClient",
        lambda config: pytest.fail("Disabled processor created a client"),
    )
    processor = UpgradeProcessor(replace(settings, upgrade_scheduler_enabled=False))
    await processor.initialize()
    with pytest.raises(AuthError) as error:
        await processor.process_one(Principal("alice"))
    assert error.value.code == "UPGRADES_DISABLED"


async def test_id_token_hint_is_explicit_and_not_sent_as_login_hint(settings, monkeypatch):
    tool_client(monkeypatch, {"id": "upgrade-one", "userId": "alice", "idToken": "identity-hint"})
    processor = UpgradeProcessor(replace(settings, ciba_hint_type="id_token_hint"))
    await processor.initialize()
    await processor.process_one(Principal("alice"))
    assert processor._client.initiation["id_token_hint"] == "identity-hint"
    assert "login_hint" not in processor._client.initiation


@pytest.mark.parametrize("result", [{}, {"error": "failed"}, {"status": "pending"}])
async def test_unknown_tool_outcome_is_not_reported_as_processed(settings, monkeypatch, result):
    tool_client(
        monkeypatch, {"id": "upgrade-one", "userId": "alice", "email": "alice@example.test"}, result
    )
    processor = UpgradeProcessor(settings)
    await processor.initialize()
    with pytest.raises(AuthError) as error:
        await processor.process_one(Principal("alice"))
    assert error.value.code == "INVALID_TOOL_RESPONSE"
    assert processor._client.cleaned == 1
