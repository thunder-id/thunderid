"""App policy tests; OAuth details stay in the SDK's own security tests."""

from __future__ import annotations

import asyncio
import time
from types import SimpleNamespace
from typing import ClassVar

import jwt
import pytest
from cryptography.hazmat.primitives.asymmetric import rsa
from thunderid import (
    AuthorizationRequest,
    Session,
    ThunderIDRuntimeError,
    TokenResponse,
)

from wayfinder_agent import auth
from wayfinder_agent.auth import AuthError, BearerVerifier, ConciergeAuth, Principal
from wayfinder_agent.settings import Settings


@pytest.fixture
def settings():
    return Settings(
        thunderid_base_url="https://issuer.example",
        agent_id="concierge",
        agent_secret="credential",
        mcp_server_url="https://booking.example/mcp",
        agent_redirect_uri="https://travel.example/agent-callback",
        chat_resource_url="https://concierge.example/chat",
        frontend_origin="https://travel.example",
    )


class FakeClient:
    instances: ClassVar[list] = []
    subject = "alice"

    def __init__(self, config):
        self.config = config
        self.initialized = False
        self.closed = False
        self.signed_out = False
        self.callback = None
        self.sign_in_calls = 0
        self.tokens = TokenResponse("access", expires_at=time.time() + 300)
        self.request = AuthorizationRequest("https://issuer.example/authorize", "owned-sdk-state")
        self.instances.append(self)

    async def initialize(self):
        self.initialized = True

    async def get_agent_token(self, **kwargs):
        self.agent_args = kwargs
        return self.tokens

    async def sign_in(self):
        self.sign_in_calls += 1
        return self.request

    async def handle_redirect_callback(self, **kwargs):
        self.callback = kwargs
        if kwargs.get("error") or kwargs.get("state") != self.request.state:
            raise ThunderIDRuntimeError("STATE_MISMATCH", "Authorization failed.")
        self.session = Session(self.tokens, {"sub": self.subject})
        return self.session

    async def get_obo_token(self, *, authorization):
        assert authorization is self.session
        return self.tokens

    async def clear_session(self):
        self.cleared = True

    async def sign_out(self):
        self.signed_out = True

    async def aclose(self):
        self.closed = True


@pytest.fixture
def fake_sdk(monkeypatch):
    FakeClient.instances = []
    FakeClient.subject = "alice"
    monkeypatch.setattr(auth, "AsyncThunderIDClient", FakeClient)
    return FakeClient


async def test_agent_token_uses_sdk_resource_and_scopes(settings, fake_sdk):
    facade = ConciergeAuth(settings)
    with pytest.raises(AuthError) as error:
        await facade.own_token()
    assert error.value.status_code == 503
    await facade.initialize()
    token = await facade.own_token()
    machine = fake_sdk.instances[0]
    assert token is machine.tokens
    assert machine.agent_args == {
        "resource": settings.mcp_server_url,
        "scopes": list(auth.OWN_SCOPES),
    }
    assert machine.config.grant_type == "client_credentials"
    await facade.aclose()
    assert machine.signed_out and machine.closed


async def test_chat_owner_isolation_and_sdk_consent(settings, fake_sdk):
    facade = ConciergeAuth(settings)
    await facade.initialize()
    context = await facade.context(Principal("alice"))
    assert await facade.context(Principal("alice"), context.id) is context
    with pytest.raises(AuthError) as error:
        await facade.context(Principal("bob"), context.id)
    assert error.value.status_code == 403
    assert await facade.user_token(context) is None
    consent = await facade.begin_consent(context)
    assert consent == await facade.begin_consent(context)
    assert consent["type"] == "need_user_consent"
    assert consent["authorize_url"] == context.client.request.url
    assert context.client.config.sign_in_options["resource"] == settings.mcp_server_url
    assert context.client.config.scopes == auth.OBO_SCOPES
    result = await facade.finish_consent(context.id, code="auth-code", state=consent["state"])
    assert result is context
    assert context.client.callback == {
        "code": "auth-code",
        "state": "owned-sdk-state",
        "error": None,
    }
    assert await facade.user_token(context) is context.client.tokens
    with pytest.raises(AuthError):
        await facade.finish_consent(context.id, code="replay", state=consent["state"])
    await facade.aclose()
    assert context.client.closed and context.client.signed_out


@pytest.mark.parametrize(
    "response",
    [
        {"code": "code"},
        {"code": "code", "state": "wrong"},
        {"code": "code", "state": "owned-sdk-state", "request_id": "wrong"},
        {"error": "access_denied", "state": "owned-sdk-state"},
    ],
)
async def test_missing_invalid_or_denied_consent(settings, fake_sdk, response):
    facade = ConciergeAuth(settings)
    await facade.initialize()
    context = await facade.context(Principal("alice"))
    await facade.begin_consent(context)
    with pytest.raises(AuthError) as error:
        await facade.finish_consent(context.id, **response)
    assert error.value.status_code == 400
    assert context.authorization is None
    await facade.aclose()


async def test_other_user_consent_is_revoked_and_rejected(settings, fake_sdk):
    facade = ConciergeAuth(settings)
    await facade.initialize()
    context = await facade.context(Principal("alice"))
    consent = await facade.begin_consent(context)
    fake_sdk.subject = "bob"
    with pytest.raises(AuthError) as error:
        await facade.finish_consent(
            context.id,
            code="code",
            state=consent["state"],
            request_id=consent["request_id"],
        )
    assert error.value.status_code == 403
    assert context.client.signed_out
    assert context.authorization is None
    assert await facade.user_token(context) is None
    await facade.aclose()


async def test_context_expiry_and_closure(settings, fake_sdk):
    facade = ConciergeAuth(settings)
    await facade.initialize()
    context = await facade.context(Principal("alice"))
    context.last_activity -= auth.SESSION_TTL + 1
    with pytest.raises(AuthError):
        await facade.context(Principal("alice"), context.id)
    await facade.expire_contexts()
    assert context.closed and context.client.closed and context.client.signed_out
    with pytest.raises(AuthError):
        await facade.begin_consent(context)
    await facade.aclose()


@pytest.fixture
def signed_bearer(settings, monkeypatch):
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    verifier = BearerVerifier(settings)
    monkeypatch.setattr(
        verifier._keys,
        "get_signing_key_from_jwt",
        lambda token: SimpleNamespace(key=key.public_key()),
    )

    def sign(**changes):
        claims = {
            "iss": settings.thunderid_base_url,
            "aud": settings.chat_resource_url,
            "iat": int(time.time()),
            "exp": int(time.time()) + 300,
            "sub": "alice",
            "scope": "agent:access",
        }
        claims.update(changes)
        return jwt.encode(claims, key, algorithm="RS256")

    return verifier, sign


async def test_verified_bearer_and_fetch_does_not_block_event_loop(signed_bearer, monkeypatch):
    verifier, sign = signed_bearer
    original = verifier._keys.get_signing_key_from_jwt
    started = asyncio.Event()
    loop = asyncio.get_running_loop()

    def slow_lookup(token):
        loop.call_soon_threadsafe(started.set)
        time.sleep(0.05)
        return original(token)

    monkeypatch.setattr(verifier._keys, "get_signing_key_from_jwt", slow_lookup)
    result = asyncio.create_task(verifier.verify("Bearer " + sign()))
    await started.wait()
    assert not result.done()
    assert (await result).subject == "alice"


@pytest.mark.parametrize(
    "header",
    [
        None,
        "",
        "Basic credential",
        "Bearer ",
        "Bearer invalid",
        "Bearer has whitespace",
    ],
)
async def test_missing_or_malformed_bearer(settings, header):
    with pytest.raises(AuthError) as error:
        await BearerVerifier(settings).verify(header)
    assert error.value.status_code == 401
    assert "credential" not in str(error.value)


@pytest.mark.parametrize(
    "claims",
    [
        {"iss": "https://other.example"},
        {"aud": "https://other.example"},
        {"exp": int(time.time()) - 10},
        {"sub": ""},
        {"scope": ["agent:access"]},
    ],
)
async def test_invalid_bearer_claims_rejected(signed_bearer, claims):
    verifier, sign = signed_bearer
    with pytest.raises(AuthError) as error:
        await verifier.verify("Bearer " + sign(**claims))
    assert error.value.status_code == 401


async def test_missing_scope_and_forged_signature_rejected(signed_bearer):
    verifier, sign = signed_bearer
    with pytest.raises(AuthError) as error:
        await verifier.verify("Bearer " + sign(scope="booking:read"))
    assert error.value.status_code == 403
    other_key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    forged = jwt.encode(
        {
            "iss": verifier._settings.thunderid_base_url,
            "aud": verifier._settings.chat_resource_url,
            "sub": "alice",
            "iat": int(time.time()),
            "exp": int(time.time()) + 300,
            "scope": "agent:access",
        },
        other_key,
        algorithm="RS256",
    )
    with pytest.raises(AuthError) as error:
        await verifier.verify("Bearer " + forged)
    assert error.value.status_code == 401


def test_settings_no_secrets_in_repr_and_explicit_urls(settings):
    environment = {
        "THUNDERID_BASE_URL": settings.thunderid_base_url,
        "AGENT_ID": settings.agent_id,
        "AGENT_SECRET": "sensitive-agent",
        "MCP_SERVER_URL": settings.mcp_server_url,
        "AGENT_REDIRECT_URI": settings.agent_redirect_uri,
        "AGENT_CHAT_URL": settings.chat_resource_url,
        "FRONTEND_ORIGIN": settings.frontend_origin,
        "LLM_API_KEY": "sensitive-model",
    }
    resolved = Settings.from_env(environment)
    assert "sensitive" not in repr(resolved)
    assert resolved.tls_context().check_hostname
    with pytest.raises(ValueError):
        Settings.from_env({})
    with pytest.raises(ValueError):
        Settings.from_env({**environment, "AGENT_PORT": "bad"})
    assert Settings.from_env({**environment, "AGENT_PORT": "8791", "PORT": "8787"}).port == 8791


async def test_sdk_terminal_authorization_failure_starts_new_consent(
    settings, fake_sdk, monkeypatch
):
    facade = ConciergeAuth(settings)
    await facade.initialize()
    context = await facade.context(Principal("alice"))
    consent = await facade.begin_consent(context)
    await facade.finish_consent(context.id, code="code", state=consent["state"])

    async def expired(*, authorization):
        raise ThunderIDRuntimeError("SESSION_EXPIRED", "The session expired.")

    monkeypatch.setattr(context.client, "get_obo_token", expired)
    assert await facade.user_token(context) is None
    assert context.authorization is None
    assert context.client.cleared
    assert (await facade.begin_consent(context))["type"] == "need_user_consent"
    await facade.aclose()


async def test_wrong_state_preserves_rightful_pending_consent(settings, fake_sdk):
    facade = ConciergeAuth(settings)
    await facade.initialize()
    context = await facade.context(Principal("alice"))
    consent = await facade.begin_consent(context)
    with pytest.raises(AuthError):
        await facade.finish_consent(context.id, code="unrelated-code", state="unrelated-state")
    assert context.client.callback is None
    assert await facade.begin_consent(context) == consent
    await facade.finish_consent(context.id, code="rightful-code", state=consent["state"])
    assert context.authorization is not None
    await facade.aclose()


async def test_cancelled_callback_drops_consumed_app_pending(settings, fake_sdk, monkeypatch):
    facade = ConciergeAuth(settings)
    await facade.initialize()
    context = await facade.context(Principal("alice"))
    first = await facade.begin_consent(context)

    async def cancelled(**kwargs):
        raise asyncio.CancelledError

    monkeypatch.setattr(context.client, "handle_redirect_callback", cancelled)
    with pytest.raises(asyncio.CancelledError):
        await facade.finish_consent(context.id, code="code", state=first["state"])
    assert context.pending is None
    assert (await facade.begin_consent(context))["request_id"] != first["request_id"]
    assert context.client.sign_in_calls == 2
    await facade.aclose()


@pytest.mark.parametrize("scopes", ["", " ", "\t\n"])
def test_blank_required_scope_cannot_disable_access_policy(settings, scopes):
    from dataclasses import replace

    with pytest.raises(ValueError, match="AGENT_ACCESS_SCOPE"):
        replace(settings, agent_access_scope=scopes)


async def test_context_creation_capacity_reserved_across_initialization(
    settings, fake_sdk, monkeypatch
):
    facade = ConciergeAuth(settings)
    await facade.initialize()
    started, release = asyncio.Event(), asyncio.Event()
    original = FakeClient.initialize

    async def blocked(client):
        started.set()
        await release.wait()
        await original(client)

    monkeypatch.setattr(FakeClient, "initialize", blocked)
    monkeypatch.setattr(auth, "MAX_CONTEXTS", 1)
    first = asyncio.create_task(facade.context(Principal("alice")))
    await started.wait()
    second = asyncio.create_task(facade.context(Principal("alice")))
    await asyncio.sleep(0)
    release.set()
    await first
    with pytest.raises(AuthError) as error:
        await second
    assert error.value.code == "CAPACITY_EXCEEDED"
    await facade.aclose()


async def test_context_cannot_register_after_shutdown(settings, fake_sdk, monkeypatch):
    facade = ConciergeAuth(settings)
    await facade.initialize()
    started, release = asyncio.Event(), asyncio.Event()
    original = FakeClient.initialize

    async def blocked(client):
        started.set()
        await release.wait()
        await original(client)

    monkeypatch.setattr(FakeClient, "initialize", blocked)
    creation = asyncio.create_task(facade.context(Principal("alice")))
    await started.wait()
    shutdown = asyncio.create_task(facade.aclose())
    await asyncio.sleep(0)
    release.set()
    with pytest.raises(AuthError) as error:
        await creation
    assert error.value.code == "NOT_INITIALIZED"
    await shutdown
    assert all(client.closed for client in FakeClient.instances)
    with pytest.raises(AuthError):
        await facade.context(Principal("alice"))
