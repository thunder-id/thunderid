"""Environment configuration for the Python Wayfinder Concierge."""

from __future__ import annotations

import os
import ssl
from collections.abc import Mapping
from dataclasses import dataclass, field
from urllib.parse import urlsplit


def _required(environment: Mapping[str, str], name: str) -> str:
    value = environment.get(name, "").strip()
    if not value:
        raise ValueError(f"Configure {name} before starting the Concierge.")
    return value


def _absolute_url(value: str, name: str) -> str:
    try:
        parsed = urlsplit(value)
        if (
            parsed.scheme not in ("http", "https")
            or not parsed.hostname
            or parsed.username is not None
            or parsed.password is not None
            or parsed.fragment
            or any(ord(char) <= 32 for char in value)
        ):
            raise ValueError
        if parsed.port is not None and not 1 <= parsed.port <= 65535:
            raise ValueError
    except ValueError:
        raise ValueError(f"Configure a valid {name} URL.") from None
    return value


@dataclass(frozen=True)
class Settings:
    thunderid_base_url: str
    agent_id: str
    agent_secret: str = field(repr=False)
    mcp_server_url: str
    agent_redirect_uri: str
    chat_resource_url: str
    frontend_origin: str
    thunderid_ca_file: str | None = None
    agent_access_scope: str = "agent:access"
    host: str = "127.0.0.1"
    port: int = 8790
    llm_provider: str = "google"
    llm_api_key: str = field(default="", repr=False)
    model_name: str = ""
    upgrade_agent_id: str = ""
    upgrade_agent_secret: str = field(default="", repr=False)
    upgrade_scheduler_enabled: bool = False
    ciba_hint_type: str = "login_hint"

    def __post_init__(self) -> None:
        if not isinstance(self.agent_access_scope, str):
            raise TypeError("Configure a nonempty AGENT_ACCESS_SCOPE.")
        scopes = self.agent_access_scope.split()
        if not scopes or any(
            any(ord(char) < 33 or ord(char) > 126 or char in '\\"' for char in scope)
            for scope in scopes
        ):
            raise ValueError("Configure a nonempty AGENT_ACCESS_SCOPE.")
        object.__setattr__(self, "agent_access_scope", " ".join(dict.fromkeys(scopes)))

    @classmethod
    def from_env(cls, environment: Mapping[str, str] | None = None) -> Settings:
        env = os.environ if environment is None else environment
        required = {
            name: _required(env, key)
            for name, key in (
                ("thunderid_base_url", "THUNDERID_BASE_URL"),
                ("agent_id", "AGENT_ID"),
                ("agent_secret", "AGENT_SECRET"),
                ("mcp_server_url", "MCP_SERVER_URL"),
                ("agent_redirect_uri", "AGENT_REDIRECT_URI"),
                ("chat_resource_url", "AGENT_CHAT_URL"),
                ("frontend_origin", "FRONTEND_ORIGIN"),
            )
        }
        for name in (
            "thunderid_base_url",
            "mcp_server_url",
            "agent_redirect_uri",
            "chat_resource_url",
            "frontend_origin",
        ):
            _absolute_url(required[name], name)
        chat = urlsplit(required["chat_resource_url"])
        try:
            port = int(
                env.get("AGENT_PORT")
                or env.get("PORT")
                or chat.port
                or (443 if chat.scheme == "https" else 80)
            )
            if not 1 <= port <= 65535:
                raise ValueError
        except ValueError:
            raise ValueError("Configure a valid AGENT_PORT.") from None
        return cls(
            thunderid_base_url=required["thunderid_base_url"],
            agent_id=required["agent_id"],
            agent_secret=required["agent_secret"],
            mcp_server_url=required["mcp_server_url"],
            agent_redirect_uri=required["agent_redirect_uri"],
            chat_resource_url=required["chat_resource_url"],
            frontend_origin=required["frontend_origin"],
            thunderid_ca_file=env.get("THUNDERID_CA_FILE") or None,
            agent_access_scope=env.get("AGENT_ACCESS_SCOPE", "agent:access"),
            host=env.get("HOST") or "127.0.0.1",
            port=port,
            llm_provider=env.get("LLM_PROVIDER", "google"),
            llm_api_key=env.get("LLM_API_KEY", ""),
            model_name=env.get("MODEL_NAME", ""),
            upgrade_agent_id=env.get("UPGRADE_AGENT_ID", ""),
            upgrade_agent_secret=env.get("UPGRADE_AGENT_SECRET", ""),
            upgrade_scheduler_enabled=env.get("UPGRADE_SCHEDULER_ENABLED", "false").lower()
            == "true",
            ciba_hint_type=env.get("CIBA_HINT_TYPE", "login_hint"),
        )

    def tls_context(self) -> ssl.SSLContext:
        return ssl.create_default_context(cafile=self.thunderid_ca_file)
