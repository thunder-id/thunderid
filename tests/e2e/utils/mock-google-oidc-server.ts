// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { IncomingMessage, ServerResponse } from "http";
import { createPrivateKey, createPublicKey, createSign, KeyObject, randomBytes } from "crypto";
import { readFileSync } from "fs";
import { join } from "path";
import { URL, URLSearchParams } from "url";
import { MockHttpServer } from "./mock-server/base";

/**
 * Mock Google user profile, matching the claim names Google's real userinfo/ID token use.
 */
export interface GoogleUserInfo {
  sub: string;
  email: string;
  emailVerified?: boolean;
  name?: string;
  givenName?: string;
  familyName?: string;
  picture?: string;
  locale?: string;
  /** Hosted domain, only present for Google Workspace accounts. */
  hd?: string;
}

interface AuthCodeData {
  email: string;
  scopes: string[];
  state: string;
  nonce: string;
  redirectUri: string;
  expiresAt: number;
}

interface AccessTokenData {
  idToken: string;
  email: string;
  expiresAt: number;
}

const KEY_ID = "mock-key-id";
const ISSUER = "accounts.google.com";
const CODE_TTL_MS = 10 * 60 * 1000;
const TOKEN_TTL_MS = 60 * 60 * 1000;

/**
 * Mock Google OIDC Server for E2E Testing
 *
 * A real HTTP server implementing Google's OIDC surface (discovery, authorize, token,
 * userinfo, JWKS) on Google's actual paths, so pointing the backend's
 * `identity_provider.google_base_url` at this server's URL redirects every Google
 * endpoint here in one step (see backend/internal/idp/utils.go::resolveEndpointDefaults,
 * which preserves paths and only rewrites scheme+host).
 *
 * Mirrors tests/integration/testutils/mock_google_oidc_server.go so the Go integration
 * suite and this Playwright suite exercise the backend identically. Reads the Go mock's
 * own key file (tests/integration/testutils/testdata/mock_google_oidc_key.pem) rather than
 * a copy, for the same reason the Go mock embeds it: a fixed `kid` must map to the same
 * key material every time it's served, or a backend JWKS cache surviving across mock
 * restarts on the same port would fail to verify. A single shared file keeps both suites'
 * `kid` in sync automatically if the key is ever rotated.
 *
 * @example
 * ```typescript
 * const mockGoogle = new MockGoogleOIDCServer(8093, "test-client-id", "test-client-secret");
 * mockGoogle.addUser({ sub: "user-1", email: "user@example.com" });
 * await mockGoogle.start();
 *
 * // ... configure a Google connection with clientId "test-client-id" and
 * // identity_provider.google_base_url = mockGoogle.getURL(), then drive the login UI.
 *
 * await mockGoogle.stop();
 * ```
 */
export class MockGoogleOIDCServer extends MockHttpServer {
  protected readonly logPrefix = "[Mock Google OIDC Server]";
  private readonly clientId: string;
  private readonly clientSecret: string;
  private readonly privateKey: KeyObject;
  private readonly publicJwk: Record<string, string>;
  private readonly users = new Map<string, GoogleUserInfo>();
  private readonly authCodes = new Map<string, AuthCodeData>();
  private readonly accessTokens = new Map<string, AccessTokenData>();
  private authorizeError: string | null = null;
  private authorizeEmail: string | null = null;

  constructor(port: number, clientId: string, clientSecret: string) {
    super(port);
    this.clientId = clientId;
    this.clientSecret = clientSecret;

    const pem = readFileSync(
      join(__dirname, "..", "..", "integration", "testutils", "testdata", "mock_google_oidc_key.pem")
    );
    this.privateKey = createPrivateKey(pem);
    const jwk = createPublicKey(this.privateKey).export({ format: "jwk" }) as Record<string, string>;
    this.publicJwk = { kty: jwk.kty, n: jwk.n, e: jwk.e, use: "sig", alg: "RS256", kid: KEY_ID };
  }

  /**
   * Register a user the authorize endpoint can sign in as. When no user has been added,
   * the authorize endpoint signs in a default "test@example.com" user. With multiple
   * users registered, the authorize endpoint signs in the first-added one unless
   * `setAuthorizeEmail` picks a specific one.
   */
  addUser(user: GoogleUserInfo): void {
    this.users.set(user.email, user);
  }

  /**
   * Force the next authorize request to sign in as a specific already-registered user,
   * for testing scenarios with more than one user added (e.g. account selection).
   * Reset to null (the default) after being consumed once.
   */
  setAuthorizeEmail(email: string | null): void {
    this.authorizeEmail = email;
  }

  /**
   * Force the next authorize request to redirect back with an OAuth `error` param
   * instead of a `code` (e.g. "access_denied"), for testing failed-login scenarios.
   * Reset to null (the default) after being consumed once.
   */
  setAuthorizeError(error: string | null): void {
    this.authorizeError = error;
  }

  private sendOAuthError(res: ServerResponse, status: number, error: string, description: string): void {
    this.sendJSON(res, status, { error, error_description: description });
  }

  protected async handleRequest(req: IncomingMessage, res: ServerResponse): Promise<void> {
    const url = new URL(req.url ?? "/", this.getURL());

    if (req.method === "GET" && url.pathname === "/.well-known/openid-configuration") {
      this.handleDiscovery(res);
    } else if (req.method === "GET" && url.pathname === "/o/oauth2/v2/auth") {
      this.handleAuthorize(url, res);
    } else if (req.method === "POST" && url.pathname === "/token") {
      await this.handleToken(req, res);
    } else if (req.method === "GET" && url.pathname === "/v1/userinfo") {
      this.handleUserInfo(req, res);
    } else if (req.method === "GET" && url.pathname === "/oauth2/v3/certs") {
      this.handleJWKS(res);
    } else {
      this.sendJSON(res, 404, { error: "not_found" });
    }
  }

  private handleDiscovery(res: ServerResponse): void {
    const baseURL = this.getURL();
    this.sendJSON(res, 200, {
      issuer: ISSUER,
      authorization_endpoint: `${baseURL}/o/oauth2/v2/auth`,
      token_endpoint: `${baseURL}/token`,
      userinfo_endpoint: `${baseURL}/v1/userinfo`,
      jwks_uri: `${baseURL}/oauth2/v3/certs`,
      response_types_supported: ["code"],
      subject_types_supported: ["public"],
      id_token_signing_alg_values_supported: ["RS256"],
      scopes_supported: ["openid", "email", "profile"],
    });
  }

  private handleAuthorize(url: URL, res: ServerResponse): void {
    const query = url.searchParams;
    const clientId = query.get("client_id");
    const redirectURI = query.get("redirect_uri");
    const state = query.get("state") ?? "";
    const nonce = query.get("nonce") ?? "";
    const responseType = query.get("response_type");

    if (!redirectURI) {
      res.writeHead(400, { "Content-Type": "text/plain" });
      res.end("Missing redirect_uri");
      return;
    }
    if (clientId !== this.clientId) {
      res.writeHead(400, { "Content-Type": "text/plain" });
      res.end("Invalid client_id");
      return;
    }
    if (responseType !== "code") {
      res.writeHead(400, { "Content-Type": "text/plain" });
      res.end("Unsupported response_type");
      return;
    }

    const redirectURL = new URL(redirectURI);
    if (state) redirectURL.searchParams.set("state", state);

    if (this.authorizeError) {
      redirectURL.searchParams.set("error", this.authorizeError);
      this.authorizeError = null;
      res.writeHead(302, { Location: redirectURL.toString() });
      res.end();
      return;
    }

    let email: string;
    if (this.authorizeEmail) {
      email = this.authorizeEmail;
      this.authorizeEmail = null;
    } else if (this.users.size === 0) {
      email = "test@example.com";
      this.addUser({ sub: "test-user-id", email, emailVerified: true, name: "Test User" });
    } else {
      email = this.users.keys().next().value as string;
    }

    const scope = query.get("scope") ?? "";
    const code = randomBytes(24).toString("hex");
    this.authCodes.set(code, {
      email,
      scopes: scope.split(" ").filter(Boolean),
      state,
      nonce,
      redirectUri: redirectURI,
      expiresAt: Date.now() + CODE_TTL_MS,
    });

    redirectURL.searchParams.set("code", code);
    res.writeHead(302, { Location: redirectURL.toString() });
    res.end();
  }

  private async handleToken(req: IncomingMessage, res: ServerResponse): Promise<void> {
    const body = await this.readBody(req);
    const form = new URLSearchParams(body);

    const grantType = form.get("grant_type");
    const code = form.get("code") ?? "";
    const clientId = form.get("client_id");
    const clientSecret = form.get("client_secret");
    const redirectURI = form.get("redirect_uri");

    if (grantType !== "authorization_code") {
      this.sendOAuthError(res, 400, "unsupported_grant_type", "Grant type not supported");
      return;
    }
    if (clientId !== this.clientId || clientSecret !== this.clientSecret) {
      this.sendOAuthError(res, 400, "invalid_client", "Invalid client credentials");
      return;
    }

    const authCode = this.authCodes.get(code);
    if (!authCode) {
      this.sendOAuthError(res, 400, "invalid_grant", "Invalid authorization code");
      return;
    }
    // Single-use: consume the code regardless of what fails below.
    this.authCodes.delete(code);

    if (Date.now() > authCode.expiresAt) {
      this.sendOAuthError(res, 400, "invalid_grant", "Authorization code expired");
      return;
    }
    if (authCode.redirectUri !== redirectURI) {
      this.sendOAuthError(res, 400, "invalid_grant", "Redirect URI mismatch");
      return;
    }

    const user = this.users.get(authCode.email);
    if (!user) {
      this.sendOAuthError(res, 400, "invalid_grant", "User not found");
      return;
    }

    const accessToken = randomBytes(32).toString("hex");
    const idToken = this.generateIDToken(user, authCode.nonce);

    this.accessTokens.set(accessToken, {
      idToken,
      email: authCode.email,
      expiresAt: Date.now() + TOKEN_TTL_MS,
    });

    this.sendJSON(res, 200, {
      access_token: accessToken,
      token_type: "Bearer",
      expires_in: TOKEN_TTL_MS / 1000,
      refresh_token: randomBytes(32).toString("hex"),
      id_token: idToken,
      scope: authCode.scopes.join(" "),
    });
  }

  private handleUserInfo(req: IncomingMessage, res: ServerResponse): void {
    const authHeader = req.headers.authorization ?? "";
    const [scheme, accessToken] = authHeader.split(" ");
    if (scheme !== "Bearer" || !accessToken) {
      res.writeHead(401, { "Content-Type": "text/plain" });
      res.end("Missing or invalid authorization header");
      return;
    }

    const tokenData = this.accessTokens.get(accessToken);
    if (!tokenData || Date.now() > tokenData.expiresAt) {
      res.writeHead(401, { "Content-Type": "text/plain" });
      res.end("Invalid or expired access token");
      return;
    }

    const user = this.users.get(tokenData.email);
    if (!user) {
      res.writeHead(500, { "Content-Type": "text/plain" });
      res.end("User not found");
      return;
    }

    this.sendJSON(res, 200, {
      sub: user.sub,
      email: user.email,
      email_verified: user.emailVerified ?? true,
      name: user.name,
      given_name: user.givenName,
      family_name: user.familyName,
      picture: user.picture,
      locale: user.locale,
      ...(user.hd ? { hd: user.hd } : {}),
    });
  }

  private handleJWKS(res: ServerResponse): void {
    this.sendJSON(res, 200, { keys: [this.publicJwk] });
  }

  private generateIDToken(user: GoogleUserInfo, nonce: string): string {
    const now = Math.floor(Date.now() / 1000);
    const header = { alg: "RS256", typ: "JWT", kid: KEY_ID };
    const claims: Record<string, unknown> = {
      iss: ISSUER,
      sub: user.sub,
      aud: this.clientId,
      exp: now + 3600,
      iat: now,
      email: user.email,
      email_verified: user.emailVerified ?? true,
      name: user.name,
      given_name: user.givenName,
      family_name: user.familyName,
      picture: user.picture,
      locale: user.locale,
    };
    if (nonce) claims.nonce = nonce;
    if (user.hd) claims.hd = user.hd;

    const headerEncoded = Buffer.from(JSON.stringify(header)).toString("base64url");
    const payloadEncoded = Buffer.from(JSON.stringify(claims)).toString("base64url");
    const signingInput = `${headerEncoded}.${payloadEncoded}`;
    const signature = createSign("RSA-SHA256").update(signingInput).sign(this.privateKey);

    return `${signingInput}.${signature.toString("base64url")}`;
  }
}
