"""SDK-owned agent credentials and isolated interactive chat authorizations."""

from __future__ import annotations

import asyncio
import math
import secrets
import time
from dataclasses import dataclass, field
from typing import Any

import jwt
from thunderid import (
    AsyncThunderIDClient,
    AuthorizationRequest,
    HttpOptions,
    MemoryStorage,
    Session,
    ThunderIDConfig,
    ThunderIDError,
    TokenRequestOptions,
    TokenResponse,
)

from .settings import Settings

OWN_SCOPES = ("booking:recommend", "upgrade:search")
OBO_SCOPES = (
    "openid",
    "booking:read",
    "booking:create",
    "booking:cancel",
    "booking:upgrade",
)
ALGORITHMS = (
    "RS256",
    "RS384",
    "RS512",
    "PS256",
    "PS384",
    "PS512",
    "ES256",
    "ES384",
    "ES512",
    "EdDSA",
)
SESSION_TTL = 30 * 60
MAX_CONTEXTS = 1000


class AuthError(Exception):
    def __init__(self, code: str, message: str, status_code: int = 401):
        super().__init__(message)
        self.code = code
        self.status_code = status_code


@dataclass(frozen=True)
class Principal:
    subject: str = field(repr=False)


class BearerVerifier:
    """Resource API policy, independent of the SDK's OAuth client operations."""

    def __init__(self, settings: Settings):
        self._settings = settings
        self._keys = jwt.PyJWKClient(
            settings.thunderid_base_url.rstrip("/") + "/oauth2/jwks",
            ssl_context=settings.tls_context(),
            timeout=10,
            lifespan=300,
        )

    async def verify(self, authorization: str | None) -> Principal:
        if not authorization or not authorization.startswith("Bearer "):
            raise AuthError("INVALID_BEARER", "A valid chat access token is required.")
        token = authorization[7:]
        if not token or len(token) > 65536 or any(char.isspace() for char in token):
            raise AuthError("INVALID_BEARER", "A valid chat access token is required.")
        return await asyncio.to_thread(self._verify, token)

    def _verify(self, token: str) -> Principal:
        try:
            header = jwt.get_unverified_header(token)
            if (
                header.get("alg") not in ALGORITHMS
                or header.get("crit")
                or "jku" in header
                or "x5u" in header
            ):
                raise jwt.InvalidTokenError()
            key = self._keys.get_signing_key_from_jwt(token)
            claims = jwt.decode(
                token,
                key.key,
                algorithms=list(ALGORITHMS),
                issuer=self._settings.thunderid_base_url,
                audience=self._settings.chat_resource_url,
                options={"require": ["iss", "aud", "exp", "iat", "sub", "scope"]},
            )
            for name in ("iat", "exp"):
                if (
                    isinstance(claims[name], bool)
                    or not isinstance(claims[name], (int, float))
                    or not math.isfinite(claims[name])
                ):
                    raise jwt.InvalidTokenError()
            if (
                not isinstance(claims["sub"], str)
                or not claims["sub"]
                or not isinstance(claims["scope"], str)
            ):
                raise jwt.InvalidTokenError()
        except (jwt.PyJWTError, ValueError, TypeError, KeyError, OSError):
            raise AuthError("INVALID_BEARER", "A valid chat access token is required.") from None
        if not set(self._settings.agent_access_scope.split()) <= set(claims["scope"].split()):
            raise AuthError("ACCESS_DENIED", "This account cannot access the Concierge.", 403)
        return Principal(claims["sub"])


@dataclass
class PendingConsent:
    authorization: AuthorizationRequest = field(repr=False)
    request_id: str = field(repr=False)


@dataclass
class ChatContext:
    id: str = field(repr=False)
    owner_sub: str = field(repr=False)
    client: AsyncThunderIDClient = field(repr=False)
    authorization: Session | None = field(default=None, repr=False)
    pending: PendingConsent | None = field(default=None, repr=False)
    last_activity: float = field(default_factory=time.monotonic)
    lock: asyncio.Lock = field(default_factory=asyncio.Lock, repr=False)
    closed: bool = False


class ConciergeAuth:
    def __init__(self, settings: Settings):
        self.settings = settings
        self._contexts: dict[str, ChatContext] = {}
        self._registry_lock = asyncio.Lock()
        self._closed = False
        self._machine = self._new_client(machine=True)
        self._initialized = False

    def _new_client(self, *, machine: bool = False) -> AsyncThunderIDClient:
        return AsyncThunderIDClient(
            ThunderIDConfig(
                base_url=self.settings.thunderid_base_url,
                client_id=self.settings.agent_id,
                client_secret=self.settings.agent_secret,
                after_sign_in_url=self.settings.agent_redirect_uri,
                grant_type="client_credentials" if machine else "authorization_code",
                scopes=OWN_SCOPES if machine else OBO_SCOPES,
                token_request=TokenRequestOptions(auth_method="client_secret_basic"),
                sign_in_options={"resource": self.settings.mcp_server_url},
                http=HttpOptions(verify=self.settings.tls_context()),
                # Chat contexts are deliberately transient; no credentials survive this process.
                storage=MemoryStorage(),
            )
        )

    async def initialize(self) -> None:
        async with self._registry_lock:
            if self._closed:
                raise AuthError("NOT_INITIALIZED", "The Concierge is not ready.", 503)
            await self._machine.initialize()
            if self._closed:
                raise AuthError("NOT_INITIALIZED", "The Concierge is not ready.", 503)
            self._initialized = True

    def _require_initialized(self) -> None:
        if not self._initialized or self._closed:
            raise AuthError("NOT_INITIALIZED", "The Concierge is not ready.", 503)

    async def own_token(self) -> TokenResponse:
        self._require_initialized()
        return await self._machine.get_agent_token(
            resource=self.settings.mcp_server_url, scopes=list(OWN_SCOPES)
        )

    async def context(self, principal: Principal, session_id: str | None = None) -> ChatContext:
        self._require_initialized()
        if session_id:
            context = self._get_context(session_id)
            if not secrets.compare_digest(context.owner_sub.encode(), principal.subject.encode()):
                raise AuthError("ACCESS_DENIED", "This chat belongs to another account.", 403)
            context.last_activity = time.monotonic()
            return context
        await self.expire_contexts()
        async with self._registry_lock:
            self._require_initialized()
            if len(self._contexts) >= MAX_CONTEXTS:
                raise AuthError("CAPACITY_EXCEEDED", "The Concierge is busy. Try again later.", 503)
            client = self._new_client()
            try:
                await client.initialize()
                self._require_initialized()
            except BaseException:
                await client.aclose()
                raise
            context = ChatContext(secrets.token_urlsafe(32), principal.subject, client)
            self._contexts[context.id] = context
            return context

    def _get_context(self, session_id: str) -> ChatContext:
        context = self._contexts.get(session_id)
        if (
            context is None
            or context.closed
            or time.monotonic() - context.last_activity >= SESSION_TTL
        ):
            raise AuthError("INVALID_SESSION", "This chat has expired. Start a new chat.", 400)
        return context

    def _check_context(self, context: ChatContext) -> None:
        if self._get_context(context.id) is not context:
            raise AuthError("INVALID_SESSION", "This chat has expired. Start a new chat.", 400)

    async def begin_consent(self, context: ChatContext) -> dict[str, Any]:
        self._require_initialized()
        async with context.lock:
            self._check_context(context)
            if context.pending is None:
                authorization = await context.client.sign_in()
                if not isinstance(authorization, AuthorizationRequest):
                    raise AuthError(
                        "INVALID_AUTHORIZATION",
                        "User authorization could not start.",
                        502,
                    )
                context.pending = PendingConsent(authorization, secrets.token_urlsafe(24))
            pending = context.pending
            return {
                "type": "need_user_consent",
                "authorize_url": pending.authorization.url,
                "state": pending.authorization.state,
                "request_id": pending.request_id,
                "scope": " ".join(OBO_SCOPES),
                "session_id": context.id,
            }

    async def finish_consent(
        self,
        session_id: str,
        *,
        code: str | None = None,
        state: str | None = None,
        request_id: str | None = None,
        error: str | None = None,
    ) -> ChatContext:
        self._require_initialized()
        if not isinstance(session_id, str) or not isinstance(state, str) or not state:
            raise AuthError("INVALID_CONSENT", "The consent response is invalid.", 400)
        context = self._get_context(session_id)
        async with context.lock:
            self._check_context(context)
            pending = context.pending
            if pending is None or (
                request_id is not None
                and (
                    not isinstance(request_id, str)
                    or not secrets.compare_digest(request_id.encode(), pending.request_id.encode())
                )
            ):
                raise AuthError("INVALID_CONSENT", "The consent response is invalid.", 400)
            if not secrets.compare_digest(state.encode(), pending.authorization.state.encode()):
                raise AuthError("INVALID_CONSENT", "The consent response is invalid.", 400)
            try:
                authorization = await context.client.handle_redirect_callback(
                    code=code, state=state, error=error
                )
            except asyncio.CancelledError:
                context.pending = None
                raise
            except ThunderIDError:
                context.pending = None
                raise AuthError(
                    "INVALID_CONSENT", "User authorization was not completed.", 400
                ) from None
            context.pending = None
            subject = authorization.user.get("sub") if authorization.user else None
            if not isinstance(subject, str) or not secrets.compare_digest(
                subject.encode(), context.owner_sub.encode()
            ):
                context.authorization = None
                try:
                    await context.client.sign_out()
                except ThunderIDError:
                    pass
                raise AuthError(
                    "ACCESS_DENIED",
                    "Authorize with the account that owns this chat.",
                    403,
                ) from None
            context.authorization = authorization
            context.last_activity = time.monotonic()
            return context

    async def user_token(self, context: ChatContext) -> TokenResponse | None:
        self._require_initialized()
        async with context.lock:
            self._check_context(context)
            if context.authorization is None:
                return None
            try:
                return await context.client.get_obo_token(authorization=context.authorization)
            except ThunderIDError as error:
                if error.code not in (
                    "NO_SESSION",
                    "SESSION_EXPIRED",
                    "INVALID_GRANT",
                    "INVALID_AUTHORIZATION",
                    "TOKEN_EXPIRED",
                ):
                    raise
                context.authorization = None
                await context.client.clear_session()
                return None

    async def close_context(self, context: ChatContext) -> None:
        async with context.lock:
            context.closed = True
            self._contexts.pop(context.id, None)
            context.authorization = None
            context.pending = None
            try:
                await context.client.sign_out()
            finally:
                await context.client.aclose()

    async def expire_contexts(self) -> None:
        expired = [
            context
            for context in self._contexts.values()
            if time.monotonic() - context.last_activity >= SESSION_TTL
        ]
        for context in expired:
            await self.close_context(context)

    async def aclose(self) -> None:
        self._closed = True
        self._initialized = False
        async with self._registry_lock:
            contexts = list(self._contexts.values())
        results = await asyncio.gather(
            *(self.close_context(context) for context in contexts),
            return_exceptions=True,
        )
        try:
            await self._machine.sign_out()
        finally:
            await self._machine.aclose()
        for result in results:
            if isinstance(result, BaseException):
                raise result
