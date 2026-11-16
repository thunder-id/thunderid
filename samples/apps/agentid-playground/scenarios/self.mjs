// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Scenario: the Concierge acts as itself.
//
// The agent authenticates with its own credentials via the client_credentials grant and asks for a
// token bound to the booking API. No user is involved, so the subject is the agent and there is no
// `act` (actor) claim. The scopes it gets back are the ones its Recommender role grants.

import {story, tokenSection, errorBox, runError} from "../lib/render.mjs";
import {checkScopes} from "../lib/readiness.mjs";
import {decodeJwt} from "../lib/jwt.mjs";

// The agent's own subject and identity claims, plus the two claims that show the token is bound.
const HIGHLIGHT = ["sub", "aud", "scope", "name", "owner", "ouId", "ouName", "ouHandle", "modelProvider"];

// What the Concierge asks for on its own behalf. booking:create is deliberately absent: browsing
// and recommending needs no user, but making a booking does.
const SCOPES = "booking:read booking:recommend";

export default {
  id: "self",
  nav: "Acts as itself",
  icon: "🧭",
  title: "The Concierge acts as itself",
  blurb: "The agent's own token, with no customer anywhere in the picture.",
  shape: ["sub = agent", "no act"],
  compare: {sub: "the Concierge", act: "none", grant: "client_credentials"},
  lead:
    "Some of what the Concierge does needs nobody's permission: browsing routes, surfacing flight " +
    "recommendations. For that it uses its own identity, and the token it receives names the agent " +
    "as the subject with no actor at all. Walk the steps below to run it against your ThunderID.",

  requires: [{key: "concierge", id: "agent-identity-concierge"}],
  resourceIds: ["agent-identity-booking-rs", "agent-identity-concierge", "agent-identity-recommender"],
  resource: {
    title: "the Concierge acting as itself",
    note: "The booking API, the Concierge, and the Recommender role that grants it booking:recommend.",
  },
  run: {
    label: "Request the Concierge's own token",
    note:
      `Calls <code>POST /oauth2/token</code> with <code>grant_type=client_credentials</code>, ` +
      `<code>scope=${SCOPES}</code>, and a <code>resource</code> naming the booking API.`,
  },

  check: ({client, config}) =>
    checkScopes(client, config.agents.concierge, {
      scope: SCOPES,
      resource: config.resources.booking,
      expect: ["booking:read", "booking:recommend"],
      role: "Recommender",
    }),

  explain: () =>
    story({
      heading: "How the Concierge acts as itself",
      items: [
        {icon: "🧭", name: "Concierge", role: "agent · subject", kind: "subject"},
        {edge: "client_credentials"},
        {icon: "🔐", name: "ThunderID", role: "authorization server"},
        {edge: "access token"},
        {icon: "🗄️", name: "Booking API", role: "resource", kind: "resource"},
      ],
      chipLabel: "Token",
      chipValues: ["sub = the Concierge", "act = none", "scope = booking:read booking:recommend"],
      note:
        "The agent authenticates with its own credentials. No customer is involved, so the token's " +
        "subject is the agent and there is no actor (<code>act</code>) claim. Because the request " +
        "names a <code>resource</code> (RFC 8707), the token's <code>aud</code> is the booking API " +
        "rather than the agent itself, and it carries only the scopes the Concierge's own " +
        "Recommender role grants. It can recommend a flight, but it cannot book one.",
    }),

  async render({client, config}) {
    let token;
    try {
      token = await client.accessToken(config.agents.concierge, {
        grant_type: "client_credentials",
        scope: SCOPES,
        resource: config.resources.booking,
      });
    } catch (err) {
      return runError(err, "If you skipped the readiness check, this is what it would have warned about. Import this tab's resource file and try again.");
    }

    const claims = decodeJwt(token)?.claims ?? {};
    const scope = String(claims.scope ?? "");
    const notice =
      claims.act !== undefined
        ? `<div class="warn">This token unexpectedly carries an <code>act</code> claim. For a pure client_credentials token there should be no actor.</div>`
        : `<div class="ok">No customer and no <code>act</code> claim: the token represents the agent itself.` +
          (scope.includes("create")
            ? ` But it carries <code>booking:create</code>, which the Recommender role should not grant.`
            : ` Note that <code>booking:create</code> is absent: the Concierge cannot book on its own.`) +
          `</div>`;

    return tokenSection({
      title: "Concierge access token (client_credentials)",
      subtitle:
        "The agent authenticated with its own credentials and received a token whose subject is the agent, bound to the booking API.",
      token,
      highlight: HIGHLIGHT,
      notice,
    });
  },
};
