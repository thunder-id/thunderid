// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Scenario: the Concierge delegates a sub-task to a worker agent.
//
// This is the multi-agent scenario from the docs: a travel-planning agent holds a token scoped to
// search, read, and book, and hands the search sub-task to a flight-search worker that gets only
// the search scope. Two things happen at every hop, and both are visible below: the token narrows,
// and the `act` (actor) claim records who carried the work.

import {story, tokenSection, chainList, section, credentials, errorBox, esc, runError} from "../lib/render.mjs";
import {checkScopes} from "../lib/readiness.mjs";
import {decodeJwt} from "../lib/jwt.mjs";
import {createLogin} from "../lib/login.mjs";

// What the Concierge holds on its own behalf, and what the worker's sub-task actually needs.
const CONCIERGE_SCOPES = "booking:read booking:recommend";
const WORKER_SCOPE = "booking:read";
// What John consents to here. The whole point is that the chain narrows it away.
const CUSTOMER_SCOPES = "openid booking:read booking:create";

// This tab signs John in itself, at its own callback and in its own session slot, so it can be
// walked from the top without visiting any other tab first.
export const login = createLogin({
  slot: "chain",
  callbackPath: "/chain/callback",
  returnPath: "/chain",
  scopes: () => CUSTOMER_SCOPES,
});

// For the terminal run, where there is no browser to sign a customer in. The act nesting is
// identical; only the subject differs, and cli.mjs says so.
export async function agentSubject(client, config) {
  return {
    token: await client.accessToken(config.agents.concierge, {
      grant_type: "client_credentials",
      scope: CONCIERGE_SCOPES,
      resource: config.resources.booking,
    }),
    who: "the Concierge (stand-in, no browser)",
  };
}

// The delegation chain is rooted in the customer who asked for the trip.
export function subjectFrom(session) {
  const token = login.state(session).tokens?.access_token;
  return token ? {token, who: "John Doe"} : null;
}

// Runs the three exchanges and returns the issued tokens. Shared with cli.mjs.
export async function runSteps(client, config, subject) {
  const {agents, resources} = config;
  const conciergeTok = await client.accessToken(agents.concierge, {
    grant_type: "client_credentials",
    scope: CONCIERGE_SCOPES,
    resource: resources.booking,
  });
  const workerTok = await client.accessToken(agents.worker, {
    grant_type: "client_credentials",
    scope: WORKER_SCOPE,
    resource: resources.booking,
  });

  const narrow = {scope: WORKER_SCOPE, resource: resources.booking};
  const singleHop = await client.exchange(agents.worker, subject.token, conciergeTok, narrow);
  const layeredActor = await client.exchange(agents.worker, workerTok, conciergeTok, narrow);
  const nestedChain = await client.exchange(agents.worker, subject.token, layeredActor, narrow);

  return [
    {
      title: "1 · The Concierge hands off the search",
      subtitle:
        `Exchange with subject = ${subject.who}, actor = the Concierge, narrowed to ` +
        `<code>${WORKER_SCOPE}</code>. One actor is recorded, and the booking scope is gone.`,
      token: singleHop,
    },
    {
      title: "2 · A layered actor token",
      subtitle:
        "Exchange with subject = the worker, actor = the Concierge. This is not a delegation " +
        "anyone performs for its own sake: it builds a token whose <code>act</code> already nests, " +
        "to be presented as the actor in the next step.",
      token: layeredActor,
    },
    {
      title: "3 · The whole path preserved",
      subtitle:
        `Exchange with subject = ${subject.who}, actor = the layered token. The customer stays the ` +
        "subject and both agents appear in the nested <code>act</code>.",
      token: nestedChain,
    },
  ];
}

const explain = () =>
  story({
    heading: "How the Concierge delegates a sub-task",
    items: [
      {icon: "👤", name: "John Doe", role: "customer · subject", kind: "subject"},
      {edge: "asks for a trip"},
      {icon: "🧭", name: "Concierge", role: "primary agent", kind: "actor"},
      {edge: "delegates search<br/>(token exchange)"},
      {icon: "✈️", name: "Flight Search Agent", role: "worker agent", kind: "actor"},
      {edge: "calls"},
      {icon: "🗄️", name: "Booking API", role: "resource", kind: "resource"},
    ],
    chipLabel: "Final token",
    chipValues: ["sub = John", "act = Flight Search → Concierge", "scope = booking:read"],
    note:
      "The subject stays the customer through every hop, and each delegation records an actor, so " +
      "the final token's nested <code>act</code> carries the whole path. Authority only shrinks: " +
      "the worker's token holds <code>booking:read</code> and nothing else, so a compromised " +
      "worker cannot book a flight even though the customer could. Each exchange builds " +
      "<code>act</code> solely from the <code>actor_token</code> and never merges the subject " +
      "token's existing actor, which is why step 2 exists.",
  });

export default {
  id: "chain",
  nav: "Delegates to agents",
  icon: "✈️",
  title: "The Concierge delegates to a worker",
  blurb: "A sub-task handed to a specialist agent, with authority shrinking at the hop.",
  shape: ["sub = customer", "act = nested agents"],
  compare: {
    sub: "the customer",
    act: "the worker, then the Concierge",
    grant: "token-exchange (RFC 8693)",
  },
  lead:
    "Complex trips decompose. The Concierge takes the goal and hands the flight search to a " +
    "specialist worker agent. Each hand-off is a token exchange that narrows what the worker may " +
    "do while recording who carried the work. Walk the steps below to watch the chain build up.",

  requires: [
    {key: "concierge", id: "agent-identity-concierge"},
    {key: "worker", id: "agent-identity-flight-search"},
  ],
  resourceIds: [
    "agent-identity-agent-rs",
    "agent-identity-booking-rs",
    "agent-identity-john",
    "agent-identity-concierge",
    "agent-identity-flight-search",
    "agent-identity-chat-user",
    "agent-identity-booking-user",
    "agent-identity-recommender",
    "agent-identity-flight-searcher",
  ],
  resource: {
    title: "the Concierge delegating to a worker",
    note:
      "Both resource servers, the consent flow, John Doe, both agents, and the roles that make the" +
      " worker's authority narrower than his.",
  },
  importNote: (config) =>
    credentials({
      title: "Sign in as",
      note: "The resource file creates John Doe for you. You will need these at step 3.",
      fields: [
        {label: "Username", value: config.loginUser.username},
        {label: "Password", value: config.loginUser.password},
      ],
    }),
  run: {
    n: 4,
    label: "Run the delegation chain",
    note:
      "Mints each agent's own token, then performs the hand-off, the layered actor, and the full " +
      "chain, each narrowed to <code>booking:read</code>.",
  },

  async check({client, config}) {
    const concierge = await checkScopes(client, config.agents.concierge, {
      scope: CONCIERGE_SCOPES,
      resource: config.resources.booking,
      expect: ["booking:read", "booking:recommend"],
      role: "Playground Recommender",
    });
    if (!concierge.ok) return {ok: false, detail: `Concierge: ${concierge.detail}`};

    const worker = await checkScopes(client, config.agents.worker, {
      scope: WORKER_SCOPE,
      resource: config.resources.booking,
      expect: ["booking:read"],
      role: "Flight Searcher",
    });
    if (!worker.ok) return {ok: false, detail: `Flight Search Agent: ${worker.detail}`};

    // This tab signs a customer in at its own callback, so that has to be registered too.
    const signIn = await login.probe(client, config);
    if (!signIn.ok) return signIn;

    return {ok: true, detail: `Concierge ${concierge.detail}; worker ${worker.detail}; ${signIn.detail}`};
  },

  explain,

  routes: login.routes(errorBox),

  // The chain must be rooted in a real customer, so signing in is a step of its own rather than
  // something borrowed from another tab.
  gate: ({session}) => {
    const subject = subjectFrom(session);
    return {
      ok: Boolean(subject),
      step: {
        n: 3,
        state: subject ? "done" : "active",
        title: "Sign in as the customer",
        tag: subject ? "signed in" : "needed",
        tagKind: subject ? "ok" : "",
        note:
          "The chain starts with a person asking for something, so John signs in and consents " +
          `to <code>${CUSTOMER_SCOPES}</code> first. The exchanges below then narrow that down to ` +
          `<code>${WORKER_SCOPE}</code> for the worker.`,
        actions: subject
          ? `<span class="muted">Signed in as John Doe.</span><a class="btn" href="/chain/logout">Sign out</a>`
          : `<a class="btn primary" href="/chain/login">Sign in as John</a>`,
      },
    };
  },

  async render({client, config, session}) {
    const subject = subjectFrom(session);
    if (!subject) return "";

    let steps;
    try {
      steps = await runSteps(client, config, subject);
    } catch (err) {
      return runError(err, "If you skipped the readiness check, this is what it would have warned about. Import this tab's resource file and try again.");
    }

    const origin = section({
      title: "Chain rooted in John Doe",
      subtitle:
        "The subject below is the customer who signed in on this tab, so this is the full shape a " +
        "production deployment produces.",
      body: `<div class="ok">Every token that follows keeps John as <code>sub</code>. Only the actor chain and the scope change.</div>`,
    });

    return (
      origin +
      steps
        .map((step) => {
          const claims = decodeJwt(step.token)?.claims ?? {};
          return tokenSection({
            title: step.title,
            subtitle: step.subtitle,
            token: step.token,
            highlight: ["sub", "act", "scope", "aud"],
            notice: `<h3>Delegation chain</h3>${chainList(claims)}`,
            extra: `<h3>act claim</h3><pre>${esc(JSON.stringify(claims.act ?? null, null, 2))}</pre>`,
          });
        })
        .join("")
    );
  },
};
