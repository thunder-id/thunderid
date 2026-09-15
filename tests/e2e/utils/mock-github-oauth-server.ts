// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { IncomingMessage, ServerResponse } from "http";
import { randomBytes } from "crypto";
import { URL, URLSearchParams } from "url";
import { MockHttpServer } from "./mock-server/base";

/**
 * Mock GitHub user profile, matching the field names GitHub's /user API returns.
 * `email` is null for accounts that keep their address private - the backend then falls back
 * to the /user/emails endpoint (see backend/internal/authn/github/service.go::FetchUserInfo).
 */
export interface GitHubUserInfo {
  /** GitHub's numeric account id. The backend renames it to the `sub` claim, as a string. */
  id: number;
  login: string;
  name?: string;
  email?: string | null;
  avatarUrl?: string;
}

export interface GitHubEmail {
  email: string;
  primary: boolean;
  verified: boolean;
}

interface AuthCodeData {
  login: string;
  scopes: string[];
  redirectUri: string;
  expiresAt: number;
}

interface AccessTokenData {
  login: string;
  scopes: string[];
  expiresAt: number;
}

const CODE_TTL_MS = 10 * 60 * 1000;
const TOKEN_TTL_MS = 8 * 60 * 60 * 1000;
const EMAIL_SCOPES = ["user", "user:email"];

/**
 * Mock GitHub OAuth Server for E2E Testing
 *
 * A real HTTP server implementing GitHub's OAuth surface (authorize, token, user, user emails)
 * on GitHub's actual paths, so pointing the backend's `identity_provider.github_base_url` at
 * this server's URL redirects every GitHub endpoint here in one step (see
 * backend/internal/idp/utils.go::resolveEndpointDefaults, which preserves paths and only rewrites
 * scheme+host). That single rewrite covers both github.com and api.github.com, which is why the
 * API paths (/user, /user/emails) are served by this same server.
 *
 * Mirrors tests/integration/testutils/mock_github_oauth_server.go so the Go integration suite and
 * this Playwright suite exercise the backend identically. Unlike the Google mock there is no ID
 * token, JWKS or discovery document to serve: GitHub is plain OAuth2, and the backend identifies
 * the user from the /user response alone.
 *
 * @example
 * ```typescript
 * const mockGitHub = new MockGitHubOAuthServer(8092, "test-client-id", "test-client-secret");
 * mockGitHub.addUser({ id: 12345, login: "testuser", email: "user@example.com" });
 * await mockGitHub.start();
 *
 * // ... configure a GitHub connection with clientId "test-client-id" and
 * // identity_provider.github_base_url = mockGitHub.getURL(), then drive the login UI.
 *
 * await mockGitHub.stop();
 * ```
 */
export class MockGitHubOAuthServer extends MockHttpServer {
  protected readonly logPrefix = "[Mock GitHub OAuth Server]";
  private readonly clientId: string;
  private readonly clientSecret: string;
  private readonly users = new Map<string, GitHubUserInfo>();
  private readonly emails = new Map<string, GitHubEmail[]>();
  private readonly authCodes = new Map<string, AuthCodeData>();
  private readonly accessTokens = new Map<string, AccessTokenData>();
  private authorizeError: string | null = null;
  private authorizeLogin: string | null = null;
  private userEmailRequestCount = 0;

  constructor(port: number, clientId: string, clientSecret: string) {
    super(port);
    this.clientId = clientId;
    this.clientSecret = clientSecret;
  }

  /**
   * Register a user the authorize endpoint can sign in as, optionally with the address list its
   * /user/emails endpoint returns. When no user has been added, the authorize endpoint signs in a
   * default "testuser" account. With multiple users registered, the authorize endpoint signs in
   * the first-added one unless `setAuthorizeLogin` picks a specific one.
   */
  addUser(user: GitHubUserInfo, emails?: GitHubEmail[]): void {
    this.users.set(user.login, user);
    if (emails) {
      this.emails.set(user.login, emails);
    }
  }

  /**
   * Force the next authorize request to sign in as a specific already-registered user,
   * for testing scenarios with more than one user added. Reset to null (the default)
   * after being consumed once.
   */
  setAuthorizeLogin(login: string | null): void {
    this.authorizeLogin = login;
  }

  /**
   * Force the next authorize request to redirect back with an OAuth `error` param
   * instead of a `code` (e.g. "access_denied"), for testing failed-login scenarios.
   * Reset to null (the default) after being consumed once.
   */
  setAuthorizeError(error: string | null): void {
    this.authorizeError = error;
  }

  /**
   * How many times /user/emails has been requested. Lets a test assert the backend actually
   * made the extra address lookup for a user whose /user response carries no email.
   */
  getUserEmailRequestCount(): number {
    return this.userEmailRequestCount;
  }

  protected onInternalError(res: ServerResponse): void {
    this.sendJSON(res, 500, { message: "Internal Server Error" });
  }

  protected async handleRequest(req: IncomingMessage, res: ServerResponse): Promise<void> {
    const url = new URL(req.url ?? "/", this.getURL());

    if (req.method === "GET" && url.pathname === "/login/oauth/authorize") {
      this.handleAuthorize(url, res);
    } else if (req.method === "POST" && url.pathname === "/login/oauth/access_token") {
      await this.handleAccessToken(req, res);
    } else if (req.method === "GET" && url.pathname === "/user") {
      this.handleUser(req, res);
    } else if (req.method === "GET" && url.pathname === "/user/emails") {
      this.handleUserEmails(req, res);
    } else {
      this.sendJSON(res, 404, { message: "Not Found" });
    }
  }

  private handleAuthorize(url: URL, res: ServerResponse): void {
    const query = url.searchParams;
    const clientId = query.get("client_id");
    const redirectURI = query.get("redirect_uri");
    const state = query.get("state") ?? "";

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

    const redirectURL = new URL(redirectURI);
    if (state) redirectURL.searchParams.set("state", state);

    if (this.authorizeError) {
      redirectURL.searchParams.set("error", this.authorizeError);
      this.authorizeError = null;
      res.writeHead(302, { Location: redirectURL.toString() });
      res.end();
      return;
    }

    let login: string;
    if (this.authorizeLogin) {
      login = this.authorizeLogin;
      this.authorizeLogin = null;
    } else if (this.users.size === 0) {
      login = "testuser";
      this.addUser({ id: 12345, login, name: "Test User", email: "test@example.com" }, [
        { email: "test@example.com", primary: true, verified: true },
      ]);
    } else {
      login = this.users.keys().next().value as string;
    }

    const code = randomBytes(24).toString("hex");
    this.authCodes.set(code, {
      login,
      // GitHub documents comma-separated scopes, but the backend joins them with a space
      // (see backend/internal/authn/oauth/service.go). Accept either separator.
      scopes: (query.get("scope") ?? "").split(/[\s,]+/).filter(Boolean),
      redirectUri: redirectURI,
      expiresAt: Date.now() + CODE_TTL_MS,
    });

    redirectURL.searchParams.set("code", code);
    res.writeHead(302, { Location: redirectURL.toString() });
    res.end();
  }

  private async handleAccessToken(req: IncomingMessage, res: ServerResponse): Promise<void> {
    const form = new URLSearchParams(await this.readBody(req));

    const code = form.get("code") ?? "";
    const clientId = form.get("client_id");
    const clientSecret = form.get("client_secret");
    const redirectURI = form.get("redirect_uri");

    // GitHub answers form-encoded by default and JSON only when asked; the backend always asks
    // (buildTokenRequest sets Accept: application/json), so this mock only speaks JSON.
    if (clientId !== this.clientId || clientSecret !== this.clientSecret) {
      this.sendJSON(res, 400, {
        error: "bad_verification_code",
        error_description: "The client_id and/or client_secret passed are incorrect.",
      });
      return;
    }

    const authCode = this.authCodes.get(code);
    if (!authCode) {
      this.sendJSON(res, 400, {
        error: "bad_verification_code",
        error_description: "The code passed is incorrect or expired.",
      });
      return;
    }
    // Single-use: consume the code regardless of what fails below.
    this.authCodes.delete(code);

    if (Date.now() > authCode.expiresAt) {
      this.sendJSON(res, 400, {
        error: "bad_verification_code",
        error_description: "The code passed is incorrect or expired.",
      });
      return;
    }
    if (redirectURI && authCode.redirectUri !== redirectURI) {
      this.sendJSON(res, 400, {
        error: "redirect_uri_mismatch",
        error_description: "The redirect_uri does not match.",
      });
      return;
    }

    const accessToken = `gho_${randomBytes(18).toString("hex")}`;
    this.accessTokens.set(accessToken, {
      login: authCode.login,
      scopes: authCode.scopes,
      expiresAt: Date.now() + TOKEN_TTL_MS,
    });

    this.sendJSON(res, 200, {
      access_token: accessToken,
      token_type: "bearer",
      scope: authCode.scopes.join(","),
    });
  }

  /**
   * Resolve the bearer/token credential on an API request, or write the 401 GitHub would return
   * and return null. GitHub accepts both the "token" and "Bearer" schemes; the backend sends Bearer.
   */
  private authenticate(req: IncomingMessage, res: ServerResponse): AccessTokenData | null {
    const [scheme, accessToken] = (req.headers.authorization ?? "").split(" ");
    if ((scheme !== "token" && scheme !== "Bearer") || !accessToken) {
      this.sendJSON(res, 401, { message: "Requires authentication" });
      return null;
    }

    const tokenData = this.accessTokens.get(accessToken);
    if (!tokenData || Date.now() > tokenData.expiresAt) {
      this.sendJSON(res, 401, { message: "Bad credentials" });
      return null;
    }

    return tokenData;
  }

  private handleUser(req: IncomingMessage, res: ServerResponse): void {
    const tokenData = this.authenticate(req, res);
    if (!tokenData) return;

    const user = this.users.get(tokenData.login);
    if (!user) {
      this.sendJSON(res, 500, { message: "User not found" });
      return;
    }

    this.sendJSON(res, 200, {
      id: user.id,
      login: user.login,
      name: user.name ?? null,
      email: user.email ?? null,
      avatar_url: user.avatarUrl ?? `https://avatars.githubusercontent.com/u/${user.id}?v=4`,
      html_url: `https://github.com/${user.login}`,
      type: "User",
    });
  }

  private handleUserEmails(req: IncomingMessage, res: ServerResponse): void {
    const tokenData = this.authenticate(req, res);
    if (!tokenData) return;

    this.userEmailRequestCount++;

    if (!tokenData.scopes.some(scope => EMAIL_SCOPES.includes(scope))) {
      this.sendJSON(res, 403, { message: "Must have user:email scope" });
      return;
    }

    this.sendJSON(res, 200, this.emails.get(tokenData.login) ?? []);
  }
}
