# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

"""Native LangChain agent with SDK token routing at each MCP tool call."""

from __future__ import annotations

import asyncio
import time
from collections.abc import AsyncIterator, Awaitable, Callable
from dataclasses import dataclass, field
from typing import Any

import httpx
from langchain.agents import create_agent
from langchain_core.messages import AIMessage, HumanMessage
from langchain_google_genai import ChatGoogleGenerativeAI
from langchain_mcp_adapters.client import MultiServerMCPClient
from langchain_mcp_adapters.interceptors import MCPToolCallRequest, MCPToolCallResult
from langgraph.checkpoint.memory import InMemorySaver
from thunderid import TokenResponse

from .auth import SESSION_TTL, AuthError, ChatContext, ConciergeAuth
from .settings import Settings

OWN_TOOLS = frozenset(
    {
        "search_flights",
        "recommend_bookings",
        "search_hotels",
        "get_trips",
        "get_locations",
        "find_upgrade_options",
    }
)
USER_TOOLS = frozenset(
    {
        "get_profile",
        "get_flight_bookings",
        "create_booking",
        "delete_all_bookings",
        "upgrade_booking",
        "request_upgrade",
    }
)
SYSTEM_PROMPT = """You are the Wayfinder travel Concierge. Use the travel tools for facts and actions.
Recommend flights with recommend_bookings. Use get_flight_bookings for the user's existing bookings.
Use a user-context tool only when the user asks for their data or an action on their behalf.
Ask for missing booking details before creating a booking. Never invent IDs or claim an action succeeded without a successful tool result.
For upgrades, first find_upgrade_options. If canUpgradeDirectly is true use upgrade_booking; otherwise request_upgrade.
Do not claim that a queued upgrade has completed: the separate CIBA processor requires the user's approval.
Keep responses concise. Never expose bearer tokens, authorization headers, or internal thought signatures."""


class _DiscoveryAuth(httpx.Auth):
    def __init__(self, token_provider: Callable[[], Awaitable[TokenResponse]]):
        self._token_provider = token_provider

    async def async_auth_flow(self, request: httpx.Request) -> AsyncIterator[httpx.Request]:
        if "Authorization" not in request.headers:
            token = await self._token_provider()
            request.headers["Authorization"] = "Bearer " + token.access_token
        yield request


def mcp_client(
    settings: Settings,
    token_provider: Callable[[], Awaitable[TokenResponse]],
    interceptors: list[Any],
) -> MultiServerMCPClient:
    """Use a fresh SDK token for discovery without retaining one in connection headers."""

    def factory(headers=None, timeout=None, auth=None):
        return httpx.AsyncClient(
            headers=headers,
            timeout=timeout or 30,
            auth=auth or _DiscoveryAuth(token_provider),
            verify=settings.tls_context(),
            follow_redirects=False,
            trust_env=False,
        )

    return MultiServerMCPClient(
        {
            "travel": {
                "transport": "streamable_http",
                "url": settings.mcp_server_url,
                "httpx_client_factory": factory,
            }
        },
        tool_interceptors=interceptors,
        handle_tool_errors=False,
    )


class ConsentRequired(Exception):
    """A tool batch needs user authorization before any tool in it executes."""


@dataclass
class _ChatRun:
    graph: Any
    context: ChatContext
    config: dict[str, Any]
    lock: asyncio.Lock = field(default_factory=asyncio.Lock)
    waiting_message: str | None = None
    failed: bool = False
    last_activity: float = field(default_factory=time.monotonic)


class Concierge:
    def __init__(self, settings: Settings, auth: ConciergeAuth):
        self.settings, self.auth = settings, auth
        self._runs: dict[str, _ChatRun] = {}
        self._model = None
        self._runs_lock = asyncio.Lock()

    @property
    def model_ready(self) -> bool:
        return self._model is not None

    async def initialize(self) -> None:
        if self.settings.llm_provider != "google":
            raise AuthError(
                "INVALID_MODEL_PROVIDER", "This sample supports the Google Gemini provider.", 503
            )
        if self.settings.llm_api_key and self.settings.model_name:
            self._model = ChatGoogleGenerativeAI(
                model=self.settings.model_name,
                api_key=self.settings.llm_api_key,
                vertexai=False,
                max_retries=0,
                timeout=60,
            )

    async def _intercept(
        self,
        context: ChatContext,
        request: MCPToolCallRequest,
        handler: Callable[[MCPToolCallRequest], Awaitable[MCPToolCallResult]],
    ) -> MCPToolCallResult:
        if request.name in USER_TOOLS:
            tokens = await self.auth.user_token(context)
            if tokens is None:
                raise ConsentRequired()
        elif request.name in OWN_TOOLS:
            tokens = await self.auth.own_token()
        else:
            raise AuthError("UNSUPPORTED_TOOL", "This tool is unavailable to the Concierge.", 403)
        headers = {"Authorization": "Bearer " + tokens.access_token}
        if request.name in USER_TOOLS and tokens.id_token:
            headers["x-id-token"] = tokens.id_token
        return await handler(request.override(headers=headers))

    async def _new_run(self, context: ChatContext) -> _ChatRun:
        async def intercept(request, handler):
            return await self._intercept(context, request, handler)

        client = mcp_client(self.settings, self.auth.own_token, [intercept])
        tools = [tool for tool in await client.get_tools() if tool.name in OWN_TOOLS | USER_TOOLS]
        if not tools:
            raise AuthError("MCP_UNAVAILABLE", "The travel tools are unavailable.", 503)
        graph = create_agent(
            self._model,
            tools=tools,
            system_prompt=SYSTEM_PROMPT,
            checkpointer=InMemorySaver(),
            interrupt_before=["tools"],
        )
        return _ChatRun(
            graph, context, {"configurable": {"thread_id": context.id}, "recursion_limit": 40}
        )

    async def _invoke(self, run: _ChatRun, value: dict[str, Any] | None):
        try:
            return await run.graph.ainvoke(value, config=run.config)
        except ConsentRequired:
            run.failed = True
            raise AuthError(
                "AUTHORIZATION_CHANGED",
                "Authorization changed during the action. Check bookings before starting a new chat.",
                409,
            ) from None
        except BaseException:
            # A failed or canceled tool call may already have committed a remote mutation.
            run.failed = True
            raise

    async def reply(self, context: ChatContext, message: str) -> dict[str, Any]:
        if not self.model_ready:
            raise AuthError(
                "MODEL_NOT_CONFIGURED",
                "Configure LLM_API_KEY and MODEL_NAME to use the Concierge.",
                503,
            )
        self._runs = {
            key: run
            for key, run in self._runs.items()
            if not run.context.closed and time.monotonic() - run.last_activity < SESSION_TTL
        }
        async with self._runs_lock:
            run = self._runs.get(context.id)
            if run is None:
                run = await self._new_run(context)
                self._runs[context.id] = run
        async with run.lock:
            run.last_activity = time.monotonic()
            if run.failed:
                raise AuthError(
                    "CHAT_INTERRUPTED",
                    "This chat was interrupted. Check bookings before starting a new chat.",
                    409,
                )
            if run.waiting_message is not None:
                if message != run.waiting_message:
                    raise AuthError(
                        "PENDING_CONSENT",
                        "Finish the pending authorization before sending another message.",
                        409,
                    )
                if await self.auth.user_token(context) is None:
                    return await self.auth.begin_consent(context)
                result = await self._invoke(run, None)
            else:
                result = await self._invoke(run, {"messages": [HumanMessage(content=message)]})
            for _ in range(12):
                snapshot = await run.graph.aget_state(run.config)
                if not snapshot.next:
                    run.waiting_message = None
                    messages = result["messages"]
                    if not messages or not isinstance(messages[-1], AIMessage):
                        raise AuthError(
                            "INVALID_MODEL_RESPONSE", "The model did not return a response.", 502
                        )
                    return {
                        "type": "response",
                        "message": messages[-1].text,
                        "session_id": context.id,
                    }
                calls = snapshot.values["messages"][-1].tool_calls
                if (
                    any(call["name"] in USER_TOOLS for call in calls)
                    and await self.auth.user_token(context) is None
                ):
                    run.waiting_message = message
                    return await self.auth.begin_consent(context)
                # Resume this checkpoint, retaining all native message metadata and completed tool results.
                result = await self._invoke(run, None)
            raise AuthError("AGENT_LIMIT", "The Concierge reached its tool-call limit.", 502)

    async def aclose(self) -> None:
        self._runs.clear()
        if self._model is not None:
            await self._model.aclose()
