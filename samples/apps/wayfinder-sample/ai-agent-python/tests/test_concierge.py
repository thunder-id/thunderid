# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

"""Exercise native checkpoints and token-routing boundaries without a model service."""

from types import SimpleNamespace

import httpx
import pytest
from langchain_core.language_models.chat_models import BaseChatModel
from langchain_core.messages import AIMessage
from langchain_core.outputs import ChatGeneration, ChatResult
from langchain_core.tools import StructuredTool
from langchain_mcp_adapters.interceptors import MCPToolCallRequest
from pydantic import Field
from thunderid import TokenResponse

from wayfinder_agent import concierge as module
from wayfinder_agent.auth import AuthError
from wayfinder_agent.concierge import Concierge, ConsentRequired, _DiscoveryAuth
from wayfinder_agent.settings import Settings


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
    )


class SequenceModel(BaseChatModel):
    responses: list[AIMessage]
    seen: list = Field(default_factory=list)

    @property
    def _llm_type(self):
        return "unit-test-sequence"

    def bind_tools(self, tools, **kwargs):
        return self

    def _generate(self, messages, stop=None, run_manager=None, **kwargs):
        self.seen.append(list(messages))
        return ChatResult(generations=[ChatGeneration(message=self.responses.pop(0))])


class Auth:
    authorized = False
    calls = 0

    async def own_token(self):
        self.calls += 1
        return TokenResponse("machine-" + str(self.calls))

    async def user_token(self, context):
        return TokenResponse("user", id_token="verified-identity") if self.authorized else None

    async def begin_consent(self, context):
        return {"type": "need_user_consent", "session_id": context.id, "state": "state"}


async def test_checkpointed_consent_preserves_history_without_replaying_mutation(
    settings, monkeypatch
):
    events = []

    async def recommend():
        """Recommend available flights."""
        events.append("recommend")
        return "flight-one"

    async def book():
        """Create the requested booking."""
        events.append("book")
        return "booking-one"

    tools = [
        StructuredTool.from_function(coroutine=recommend, name="recommend_bookings"),
        StructuredTool.from_function(coroutine=book, name="create_booking"),
    ]

    class Client:
        async def get_tools(self):
            return tools

    monkeypatch.setattr(module, "mcp_client", lambda *args: Client())
    auth = Auth()
    assistant = Concierge(settings, auth)
    model = SequenceModel(
        responses=[
            AIMessage(
                content="",
                additional_kwargs={"thought_signature": "preserve-this"},
                tool_calls=[{"name": "recommend_bookings", "args": {}, "id": "recommend"}],
            ),
            AIMessage(
                content="", tool_calls=[{"name": "create_booking", "args": {}, "id": "book"}]
            ),
            AIMessage(content="Booked the flight."),
            AIMessage(content="Your booking is booking-one."),
        ]
    )
    assistant._model = model
    context = SimpleNamespace(id="session", closed=False)
    first = await assistant.reply(context, "Recommend and book a flight")
    assert first["type"] == "need_user_consent"
    assert events == ["recommend"]
    with pytest.raises(AuthError) as error:
        await assistant.reply(context, "Different action")
    assert error.value.code == "PENDING_CONSENT"
    auth.authorized = True
    resumed = await assistant.reply(context, "Recommend and book a flight")
    assert resumed["message"] == "Booked the flight."
    assert events == ["recommend", "book"]
    final = await assistant.reply(context, "What is my booking?")
    assert final["message"] == "Your booking is booking-one."
    assert events == ["recommend", "book"]
    history = model.seen[-1]
    assert (
        sum(
            message.type == "human" and message.content == "Recommend and book a flight"
            for message in history
        )
        == 1
    )
    assert any(
        message.additional_kwargs.get("thought_signature") == "preserve-this" for message in history
    )


async def test_whole_tool_batch_pauses_before_any_side_effect(settings, monkeypatch):
    events = []

    async def mutate():
        """Create a booking."""
        events.append("mutation")
        return "done"

    async def recommend():
        """Recommend a flight."""
        events.append("recommendation")
        return "flight"

    tools = [
        StructuredTool.from_function(coroutine=mutate, name="create_booking"),
        StructuredTool.from_function(coroutine=recommend, name="recommend_bookings"),
    ]

    class Client:
        async def get_tools(self):
            return tools

    monkeypatch.setattr(module, "mcp_client", lambda *args: Client())
    auth = Auth()
    assistant = Concierge(settings, auth)
    assistant._model = SequenceModel(
        responses=[
            AIMessage(
                content="",
                tool_calls=[
                    {"name": "create_booking", "args": {}, "id": "a"},
                    {"name": "recommend_bookings", "args": {}, "id": "b"},
                ],
            ),
            AIMessage(content="Done"),
        ]
    )
    context = SimpleNamespace(id="batch", closed=False)
    assert (await assistant.reply(context, "Book a flight"))["type"] == "need_user_consent"
    assert events == []
    auth.authorized = True
    await assistant.reply(context, "Book a flight")
    assert sorted(events) == ["mutation", "recommendation"]


async def test_each_tool_uses_fresh_sdk_helper_result_and_user_identity_header(settings):
    auth = Auth()
    assistant = Concierge(settings, auth)
    received = []

    async def handler(request):
        received.append(request)
        return "result"

    own = MCPToolCallRequest(name="recommend_bookings", args={}, server_name="travel")
    await assistant._intercept(None, own, handler)
    await assistant._intercept(None, own, handler)
    assert [request.headers["Authorization"] for request in received] == [
        "Bearer machine-1",
        "Bearer machine-2",
    ]
    assert own.headers is None
    user = MCPToolCallRequest(name="get_profile", args={}, server_name="travel")
    with pytest.raises(ConsentRequired):
        await assistant._intercept(None, user, handler)
    auth.authorized = True
    await assistant._intercept(None, user, handler)
    assert received[-1].headers == {
        "Authorization": "Bearer user",
        "x-id-token": "verified-identity",
    }
    with pytest.raises(AuthError):
        await assistant._intercept(
            None, MCPToolCallRequest(name="process_upgrade", args={}, server_name="travel"), handler
        )


async def test_discovery_auth_never_overwrites_per_tool_authorization():
    auth = Auth()
    seen = []

    async def transport(request):
        seen.append(request.headers["authorization"])
        return httpx.Response(200)

    async with httpx.AsyncClient(
        transport=httpx.MockTransport(transport), auth=_DiscoveryAuth(auth.own_token)
    ) as client:
        await client.get("https://booking.example/mcp")
        await client.get("https://booking.example/mcp", headers={"Authorization": "Bearer user"})
    assert seen == ["Bearer machine-1", "Bearer user"]
    assert auth.calls == 1


async def test_missing_model_configuration_never_returns_fake_reply(settings):
    assistant = Concierge(settings, Auth())
    await assistant.initialize()
    assert not assistant.model_ready
    with pytest.raises(AuthError) as error:
        await assistant.reply(SimpleNamespace(id="session"), "Hello")
    assert error.value.code == "MODEL_NOT_CONFIGURED"


async def test_authorization_vanishing_mid_batch_ends_run_without_replaying_mutation(
    settings, monkeypatch
):
    class ChangingAuth(Auth):
        checks = 0

        async def user_token(self, context):
            self.checks += 1
            return TokenResponse("user") if self.checks <= 2 else None

    auth = ChangingAuth()
    assistant = Concierge(settings, auth)
    events = []
    context = SimpleNamespace(id="authorization-race", closed=False)

    async def book():
        """Create a booking."""

        async def handler(request):
            events.append("booked")
            return "booking-one"

        return await assistant._intercept(
            context,
            MCPToolCallRequest(name="create_booking", args={}, server_name="travel"),
            handler,
        )

    async def delete():
        """Delete bookings."""

        async def handler(request):
            events.append("deleted")
            return "deleted"

        return await assistant._intercept(
            context,
            MCPToolCallRequest(name="delete_all_bookings", args={}, server_name="travel"),
            handler,
        )

    tools = [
        StructuredTool.from_function(coroutine=book, name="create_booking"),
        StructuredTool.from_function(coroutine=delete, name="delete_all_bookings"),
    ]

    class Client:
        async def get_tools(self):
            return tools

    monkeypatch.setattr(module, "mcp_client", lambda *args: Client())
    assistant._model = SequenceModel(
        responses=[
            AIMessage(
                content="",
                tool_calls=[
                    {"name": "create_booking", "args": {}, "id": "book"},
                    {"name": "delete_all_bookings", "args": {}, "id": "delete"},
                ],
            )
        ]
    )
    with pytest.raises(AuthError) as first:
        await assistant.reply(context, "Perform the requested actions")
    assert first.value.code == "AUTHORIZATION_CHANGED"
    assert events == ["booked"]
    with pytest.raises(AuthError) as retry:
        await assistant.reply(context, "Perform the requested actions")
    assert retry.value.code == "CHAT_INTERRUPTED"
    assert events == ["booked"]
