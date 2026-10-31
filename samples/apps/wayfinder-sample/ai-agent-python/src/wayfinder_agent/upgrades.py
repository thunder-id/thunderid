# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

"""Optional one-request CIBA processor, separate from the chat model."""

from __future__ import annotations

import asyncio
import json
from typing import Any

from thunderid import (
    AsyncThunderIDClient,
    HttpOptions,
    MemoryStorage,
    ThunderIDConfig,
    TokenRequestOptions,
)

from .auth import AuthError, Principal
from .concierge import mcp_client
from .settings import Settings


def _result_json(value: Any) -> dict[str, Any]:
    if hasattr(value, "content"):
        value = value.content
    if isinstance(value, list):
        value = "".join(
            item.get("text", "")
            for item in value
            if isinstance(item, dict) and item.get("type") == "text"
        )
    try:
        result = json.loads(value) if isinstance(value, str) else value
    except (ValueError, TypeError):
        raise AuthError(
            "INVALID_TOOL_RESPONSE", "The upgrade tool returned an invalid response.", 502
        ) from None
    if not isinstance(result, dict):
        raise AuthError(
            "INVALID_TOOL_RESPONSE", "The upgrade tool returned an invalid response.", 502
        )
    return result


class UpgradeProcessor:
    def __init__(self, settings: Settings):
        self.settings = settings
        self.enabled = settings.upgrade_scheduler_enabled
        self._client = None
        self._lock = asyncio.Lock()

    async def initialize(self) -> None:
        if not self.enabled:
            return
        if (
            not self.settings.upgrade_agent_id
            or not self.settings.upgrade_agent_secret
            or self.settings.ciba_hint_type not in {"login_hint", "id_token_hint"}
        ):
            raise AuthError(
                "INVALID_UPGRADE_CONFIG", "Configure the separate CIBA client and hint type.", 503
            )
        self._client = AsyncThunderIDClient(
            ThunderIDConfig(
                base_url=self.settings.thunderid_base_url,
                client_id=self.settings.upgrade_agent_id,
                client_secret=self.settings.upgrade_agent_secret,
                scopes=("openid", "upgrade:process"),
                token_request=TokenRequestOptions(auth_method="client_secret_basic"),
                http=HttpOptions(verify=self.settings.tls_context()),
                storage=MemoryStorage(),
            )
        )
        await self._client.initialize()

    async def _own_token(self):
        return await self._client.get_agent_token(
            resource=self.settings.mcp_server_url, scopes=["upgrade:read", "upgrade:search"]
        )

    async def process_one(self, principal: Principal) -> dict[str, str]:
        if not self.enabled or self._client is None:
            raise AuthError("UPGRADES_DISABLED", "The CIBA upgrade processor is disabled.", 404)
        async with self._lock:
            user_tokens = None

            async def intercept(request, handler):
                if request.name == "get_pending_upgrade":
                    token = await self._own_token()
                elif request.name == "process_upgrade" and user_tokens is not None:
                    token = await self._client.get_obo_token(authorization=user_tokens)
                else:
                    raise AuthError("UNSUPPORTED_TOOL", "This upgrade tool is unavailable.", 403)
                return await handler(
                    request.override(headers={"Authorization": "Bearer " + token.access_token})
                )

            try:
                tools = {
                    tool.name: tool
                    for tool in await mcp_client(
                        self.settings, self._own_token, [intercept]
                    ).get_tools()
                }
                if not {"get_pending_upgrade", "process_upgrade"} <= tools.keys():
                    raise AuthError("MCP_UNAVAILABLE", "The upgrade tools are unavailable.", 503)
                pending = _result_json(await tools["get_pending_upgrade"].ainvoke({})).get(
                    "request"
                )
                if pending is None:
                    return {"status": "no_pending_upgrade"}
                if not isinstance(pending, dict) or not isinstance(pending.get("id"), str):
                    raise AuthError("INVALID_TOOL_RESPONSE", "The pending upgrade is invalid.", 502)
                if pending.get("userId") != principal.subject:
                    raise AuthError(
                        "ACCESS_DENIED", "The pending upgrade belongs to another account.", 403
                    )
                hint_name = self.settings.ciba_hint_type
                hint = pending.get("idToken" if hint_name == "id_token_hint" else "email")
                if not isinstance(hint, str) or not hint:
                    raise AuthError(
                        "INVALID_UPGRADE_HINT",
                        "The pending upgrade has no configured user hint.",
                        400,
                    )
                request = await self._client.initiate_backchannel_authentication(
                    **{hint_name: hint},
                    resource=self.settings.mcp_server_url,
                    scopes=["openid", "upgrade:process"],
                    binding_message="Approve your Wayfinder flight upgrade.",
                )
                user_tokens = await self._client.await_backchannel_authentication(
                    request, timeout=120
                )
                result = _result_json(
                    await tools["process_upgrade"].ainvoke({"upgradeRequestId": pending["id"]})
                )
                if result.get("status") == "failed":
                    return {"status": "upgrade_failed"}
                if result.get("status") != "success":
                    raise AuthError("INVALID_TOOL_RESPONSE", "The upgrade result is invalid.", 502)
                return {"status": "processed"}
            finally:
                await self._client.sign_out()

    async def aclose(self) -> None:
        if self._client is not None:
            await self._client.aclose()
