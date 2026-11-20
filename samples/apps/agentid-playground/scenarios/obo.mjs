// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Scenario: the Concierge acts on behalf of John Doe.
//
// The OAuth client is an agent, so every access token issued from a user login names the user as
// `sub` and the agent as `act` (actor). That makes it an on-behalf-of token by construction, with no
// extra exchange required. The refresh and token-exchange actions then show the actor being carried
// and the authority being narrowed.
//
// This is the only scenario that needs a browser round trip, so it owns the extra routes below.
// `/callback` stays at the root because it must match the agent's registered redirect URI exactly.

import {story, tokenSection, section, credentials, errorBox, esc, runError} from "../lib/render.mjs";
import {ACCESS_TOKEN_TYPE, JWT_TYPE, EXCHANGE_GRANT} from "../lib/http.mjs";
import {decodeJwt} from "../lib/jwt.mjs";
import {createLogin} from "../lib/login.mjs";

// This tab signs John in at /callback, the redirect URI registered on the Concierge.
const login = createLogin({
  slot: "obo",
  callbackPath: "/callback",
  returnPath: "/obo",
  scopes: (config) => config.scopes,
});
const state = login.state;
const redirectUri = login.redirectUri;

const tokenRequest = (client, config, params) => client.tokenResponse(config.agents.concierge, params);

// What the exchange narrows the customer's token down to. The login asks for read and create; the
// exchange keeps only read, which is what makes the result visibly different from its input.
const EXCHANGE_SCOPE = "booking:read";

function renderStory() {
  return story({
    heading: "How the Concierge acts for a customer",
    items: [
      {icon: "👤", name: "John Doe", role: "customer · subject", kind: "subject"},
      {edge: "signs in and consents<br/>(authorization_code)"},
      {icon: "🧭", name: "Concierge", role: "agent · actor", kind: "actor"},
      {edge: "on-behalf-of token"},
      {icon: "🗄️", name: "Booking API", role: "resource", kind: "resource"},
    ],
    chipLabel: "Access token",
    chipValues: ["sub = John", "act = the Concierge", "scope = booking:read booking:create"],
    note:
      "Booking a trip needs the customer's permission, so John signs in and approves on the " +
      "consent screen. The token names John as subject (<code>sub</code>) and the Concierge as " +
      "actor (<code>act</code>), so the booking API sees both who authorized the trip and which " +
      "agent carried it out. Compare it with the Concierge's own token on the first tab: same " +
      "agent, different grant, and <code>booking:create</code> appears only here.",
  });
}

// When the token carries an `act` claim, summarize the on-behalf-of relationship: the user is the
// subject and the agent is the actor. This is what makes an agent token an on-behalf-of token.
function oboSummary(claims) {
  const act = claims.act;
  if (!act || typeof act !== "object") return "";
  return `<div class="obo">On behalf of, subject (user): <code>${esc(String(claims.sub ?? ""))}</code> &nbsp;·&nbsp; actor (agent): <code>${esc(String(act.sub ?? ""))}</code></div>`;
}

function dashboard(s) {
  const t = s.tokens;
  const banners =
    (s.refreshed
      ? `<div class="banner">Refreshed, the access token below is newly issued (compare <code>iat</code>/<code>jti</code>). The actor (<code>act</code>) is preserved.</div>`
      : "") + (s.note ? `<div class="error">${esc(s.note)}</div>` : "");

  const sessionSection = section({
    title: "Session",
    subtitle: `Granted scope: <code>${esc(t.scope ?? "")}</code> · token_type: <code>${esc(t.token_type ?? "")}</code> · expires_in: <code>${esc(t.expires_in ?? "")}</code>`,
    body: `<div class="actions">
  <form method="post" action="/obo/refresh" style="margin:0"><button type="submit">Refresh access token</button></form>
  <form method="post" action="/obo/exchange" style="margin:0"><button type="submit">Narrow to booking:read for the booking API</button></form>
  <a class="btn" href="/obo/logout">Log out</a>
</div>`,
  });

  // After a refresh these three claims are the ones that actually differ, so the table marks them
  // rather than leaving the reader to compare two long payloads by eye.
  const changedByRefresh = s.refreshed ? ["iat", "exp", "jti"] : [];

  const withSummary = (token, highlight, title, subtitle, changed = []) => {
    const claims = token ? decodeJwt(token)?.claims : null;
    const notice = claims ? oboSummary(claims) : "";
    return tokenSection({title, subtitle, token, highlight, changed, notice, showHeader: true});
  };

  return (
    banners +
    sessionSection +
    withSummary(
      t.id_token,
      ["sub", "email"],
      "ID token",
      "Issued because the openid scope was requested. Identifies the authenticated user. ID tokens never carry an act claim.",
    ) +
    withSummary(
      t.access_token,
      ["sub", "act", "scope", "aud", "email", "username"],
      "Access token (on-behalf-of)",
      "Because the client is an agent, this bearer token names John as sub and the Concierge as act (actor). It carries the booking scopes John's Playground Booking User role grants, and the user attributes userConfig.attributes allow-lists.",
      changedByRefresh,
    ) +
    tokenSection({
      title: "Refresh token",
      subtitle: "Used by the refresh_token grant to obtain fresh access tokens without re-login.",
      token: t.refresh_token,
    }) +
    (s.exchange
      ? withSummary(
          s.exchange.access_token,
          ["sub", "act", "scope", "aud"],
          "Token exchange result (on-behalf-of)",
          `Exchanged John's access token, presenting the Concierge's own token as <code>actor_token</code> and asking for <code>${EXCHANGE_SCOPE}</code> against the booking API. Compare <code>scope</code> and <code>aud</code> with the access token above: <code>booking:create</code> is gone, so this token can search but no longer book. issued_token_type: <code>${esc(s.exchange.issued_token_type ?? "")}</code>`,
        )
      : "")
  );
}

export default {
  id: "obo",
  nav: "Acts for a customer",
  icon: "👤",
  title: "The Concierge acts for a customer",
  blurb: "John signs in, consents, and the token names both him and the agent.",
  shape: ["sub = customer", "act = agent"],
  compare: {
    sub: "the signed-in customer",
    act: "the Concierge",
    grant: "authorization_code, refresh_token, token-exchange",
  },
  lead:
    "Booking a trip spends John's money, so it needs John's permission. He signs in and approves " +
    "the Concierge on a consent screen, and because the OAuth client is an <strong>agent</strong>, " +
    "the token names him as <code>sub</code> and the Concierge as <code>act</code> automatically. " +
    "Walk the steps below to sign in as John and see it.",

  requires: [{key: "concierge", id: "agent-identity-concierge"}],
  resourceIds: [
    "agent-identity-agent-rs",
    "agent-identity-booking-rs",
    "0193b0a1-2001-7000-8000-000000000001",
    "agent-identity-john",
    "agent-identity-concierge",
    "agent-identity-chat-user",
    "agent-identity-booking-user",
  ],
  resource: {
    title: "the Concierge acting for a customer",
    note:
      "Both resource servers, the consent flow, John Doe, the Concierge, and the two roles that let"
      + " John reach the agent and grant it his booking permissions.",
  },
  importNote: (config) =>
    credentials({
      title: "Sign in as",
      note:
        "The resource file creates John Doe for you. Use these on the ThunderID gate after you " +
        "click the button below. He holds <code>Playground Chat User</code>, which the consent flow checks " +
        "before it will even show him the Allow screen.",
      fields: [
        {label: "Username", value: config.loginUser.username},
        {label: "Password", value: config.loginUser.password},
      ],
    }),
  run: {
    label: "Sign in as John",
    href: "/obo/login",
    note:
      "Starts the <code>authorization_code</code> flow with PKCE (S256). You land on the ThunderID " +
      "gate branded with the Concierge's logo, enter John's credentials, and approve the consent " +
      "screen before coming back holding the tokens.",
  },

  explain: renderStory,

  // An agent that exists but is registered wrong is the common failure here, so probe the authorize
  // endpoint and name the problem rather than letting the user discover it on the gate.
  //
  // The probe deliberately omits code_challenge. A PKCE-required client then answers
  // "code_challenge is required", which already proves the client, redirect URI, and grant were all
  // accepted, and it stops before an authorization session is created.
  // An agent that exists but is registered wrong is the common failure here, so probe the real
  // authorize request and name the problem rather than letting the user find it on the gate.
  async check({client, config}) {
    // The Concierge and the booking API ship in every tab's slice, so finding them proves nothing
    // about THIS tab. The agent resource server is unique to this slice, and it travels with John
    // and his roles, so it is the one thing that distinguishes "obo imported" from "some other tab
    // imported". Without it the step would go green and the sign-in would then fail on the gate.
    for (const [label, resource] of [
      ["agent", config.resources.agent],
      ["booking", config.resources.booking],
    ]) {
      try {
        await client.accessToken(config.agents.concierge, {grant_type: "client_credentials", resource});
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        return {
          ok: false,
          detail: /invalid_target/.test(message)
            ? `${resource} is not registered, so this tab's resources are not imported yet. The ` +
              `${label} resource server ships in this tab's file, along with John and his roles.`
            : message,
        };
      }
    }

    const result = await login.probe(client, config);
    return result.ok
      ? {ok: true, detail: `${result.detail} John and his roles are only proven by signing in.`}
      : result;
  },

  hasResult: ({session}) => Boolean(state(session).tokens),

  async render({session}) {
    const s = state(session);
    return s.tokens ? dashboard(s) : "";
  },

  routes: {
    ...login.routes(errorBox),

    "POST /obo/refresh": async ({res, session, config, client, redirect}) => {
      const s = state(session);
      if (!s.tokens?.refresh_token) return redirect(res, "/obo");
      try {
        const refreshed = await tokenRequest(client, config, {
          grant_type: "refresh_token",
          refresh_token: s.tokens.refresh_token,
        });
        // Keep the prior refresh token if the server didn't issue a new one.
        s.tokens = {...s.tokens, ...refreshed, refresh_token: refreshed.refresh_token ?? s.tokens.refresh_token};
        s.refreshed = true;
        s.note = undefined;
      } catch (e) {
        s.note = `Refresh failed: ${e instanceof Error ? e.message : String(e)}`;
      }
      return redirect(res, "/obo");
    },

    "POST /obo/exchange": async ({res, session, config, client, redirect}) => {
      const s = state(session);
      if (!s.tokens?.access_token) return redirect(res, "/obo");
      try {
        // The agent mints its own token to present as the actor, so the exchanged token records the
        // agent in its act claim. Without an actor_token, token exchange omits act.
        const agentToken = await tokenRequest(client, config, {grant_type: "client_credentials"});
        s.exchange = await tokenRequest(client, config, {
          grant_type: EXCHANGE_GRANT,
          subject_token: s.tokens.access_token,
          subject_token_type: JWT_TYPE,
          actor_token: agentToken.access_token,
          actor_token_type: JWT_TYPE,
          requested_token_type: ACCESS_TOKEN_TYPE,
          // Narrowing is the reason to exchange at all. Without it the result would be
          // indistinguishable from the token it came from, since the actor is the same agent.
          scope: EXCHANGE_SCOPE,
          resource: config.resources.booking,
        });
        s.note = undefined;
      } catch (e) {
        s.note = `Token exchange failed: ${e instanceof Error ? e.message : String(e)}`;
      }
      return redirect(res, "/obo");
    },

  },
};
