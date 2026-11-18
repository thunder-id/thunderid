# ThunderID Agent Identity Playground

A hands-on playground for ThunderID's agent identity management features. It is one app covering the
three ways an AI agent can hold identity in ThunderID, each on its own tab, run live against your own
instance.

Nothing here is a recording or a mock-up. Every tab performs the real OAuth calls against the
ThunderID you point it at, then decodes and explains the tokens that come back. Import the resources
it hands you, click through a posture, and read the claims ThunderID actually issued. Change the
roles, scopes, or agents in the resource file, reload, and watch the tokens change with them.

The cast is a travel-booking one: a **Concierge** agent that recommends and books trips, a worker
agent it hands searches to, and a customer named **John Doe**. There is no chat widget and no MCP
server here, only the identity pieces and the claims they produce.

Using one cast across all three tabs is deliberate. The overview page puts them side by side, and
because the domain, the API, and the agent stay the same, the only thing that changes between tabs
is who the token names and what it is allowed to do.

| Tab | What the Concierge is doing | `sub` | `act` | `scope` |
|---|---|---|---|---|
| **Acts as itself** | Recommending flights, nobody's permission needed | the Concierge | none | `booking:read booking:recommend` |
| **Acts for a customer** | Booking a trip John asked for | John | the Concierge | `booking:read booking:create` |
| **Delegates to agents** | Handing flight search to a worker | the customer | worker, then Concierge | `booking:read` |

Read the `scope` column downward: authority is granted by role, widened only by a user's consent,
and narrowed at every delegation. The worker can search but never book, even though John can.

Every token is decoded (without signature verification) and shown in full, with the claims that
carry the identity story highlighted. Nothing is added: what you see is what ThunderID issued.

## Self-contained resources

Everything the playground imports is its own. Every id is prefixed `agent-identity-`, the resource
servers use their own identifiers under `travel.example`, and the customer is `johndoe`, so importing
it will not overwrite or collide with anything else already in your instance. Import is upsert by id,
so re-importing, or importing the per-tab slices the app offers, is always safe.

## Prerequisites

Only one: **Node 22 or later** (built-in modules only, plus `--env-file`, so no `npm install`).

You do not need to set anything up before opening the app. Every tab checks its own prerequisites
and walks you through whatever is missing.

## Run

```bash
cp .env.example .env    # optional; sets THUNDERID_BASE_URL (defaults to https://localhost:8090)
npm start
```

Then open http://localhost:8082. The app starts and explains itself whether or not ThunderID is
running and whether or not anything has been imported.

## How each tab works

Every tab has the same three parts, top to bottom:

1. **The explanation.** What this posture means, drawn as a flow, with the token shape it produces.
   Always visible, even with nothing set up.
2. **Try it out.** Checks run live against your instance on every page load. Each tab is
   self-contained: it never depends on another tab having been visited first, so you can walk any
   one of them from the top.
   - **ThunderID is running.** Any answer from the backend counts. If nothing answers, the header
     pill turns red and the step lists what to fix.
   - **Import the playground resources.** You complete this one, by clicking **I have imported it, mark
     as ready**. The step offers the exact resource slice to download, so you never import more than
     the tab needs, and **Undo** puts it back.

     The app still probes your instance and reports what it saw on the same step, as `Detected: ...`
     or `Not detected: ...`. That is information, not a gate. The **Acts for a customer** tab also
     probes the authorize endpoint, so a missing `authorization_code` grant or a mismatched redirect
     URI is named precisely rather than surfacing later on the gate.
   - **Sign in as the customer.** Only on the tabs whose story starts with a person. Both the
     Acts for a customer and Delegates to agents tabs run their own `authorization_code` flow, at
     their own callback (`/callback` and `/chain/callback`), in their own session. Signing out of
     one does not affect the other.
   - **Run it.** Unlocked once the preceding steps are complete. Clicking it performs the real calls.

   Detection informs, you decide. Some prerequisites cannot be proven without actually using them,
   John's existence being the clearest case, so nothing is auto-completed on your behalf. If you
   confirm the import and something is in fact missing, the run says so plainly: "The run failed",
   with exactly the response ThunderID returned.
3. **The claims.** Decoded tokens, appearing below the steps once you have run it.

Nothing is cached, so reload at any time to recheck. The overview page offers the combined resource
file if you would rather import everything up front.

### Extra prerequisites for the Acts for a customer tab

It is the only tab that opens a browser login, so it also needs the **Gate UI running**, since
`authorization_code` redirects there. The normal dev stack (`make run`, or the console and gate dev
servers) covers this.

You do not need to bring your own user. The resource file creates **John Doe** for you, of the
built-in `Person` user type, and the tab shows his credentials with a copy button:

```
username: johndoe
password: Travel@123
```

The password is in the resource file in the clear because everything here runs locally.

Every sign-in sends `prompt=consent`, so the Allow screen appears every time. Without it ThunderID
remembers a granted consent and silently skips the prompt on later logins, which is right for
production and wrong here.

John holds two roles. `Chat User` grants `agent:access`, which the consent flow checks before it
will show him the Allow screen, and `Booking User` grants the `booking:read` and `booking:create`
that end up in his token. A user without `agent:access` is rejected before consent, which is the
"Protect the Agent" behaviour from the docs.

To skip that tab entirely: `SCENARIOS=self,chain npm start`.

The playground connects to the local self-signed ThunderID over TLS without verifying the certificate,
scoped to its own request agent (localhost only), so there is no global TLS warning.

## Configure

`config.json` holds where the app serves itself, the scopes, and the agent credentials, which must
match `thunderid-config/thunderid-config.yaml`:

```json
{
  "thunderidBaseUrl": "https://localhost:8090",
  "appBaseUrl": "http://localhost:8082",
  "scopes": "openid profile email booking:read booking:create",
  "resources": {
    "agent": "https://agent.travel.example",
    "booking": "https://api.travel.example/booking"
  },
  "loginUser": { "username": "johndoe", "password": "Travel@123" },
  "agents": { "concierge": {...}, "worker": {...} }
}
```

`scopes` is what the login asks for on John's behalf. The `resources` entries are RFC 8707 resource
indicators that bind tokens to a specific API, and each must match a resource server's `identifier`
in the resource file. They double as readiness probes: an unregistered resource is rejected with
`invalid_target`, which is how a tab tells whether its own slice was imported. `loginUser` is shown on the Acts for a customer tab so you can copy the
credentials; it is display only, the real user comes from the import.

The environment (`.env`, loaded automatically) overrides parts of it:

| Variable | Effect |
|---|---|
| `THUNDERID_BASE_URL` | The ThunderID backend. Overrides `thunderidBaseUrl`. |
| `SCENARIOS` | Comma-separated tab ids to enable: `self`, `obo`, `chain`. Defaults to all three. |
| `PORT` | Port for this app. Defaults to the port in `appBaseUrl`. |

`openid` in `scopes` is what makes ThunderID issue an ID token.

Changing the port while the `obo` tab is enabled means also changing the agent's registered
`redirectUris` in `thunderid-config/thunderid-config.yaml`, since the callback must match exactly.
Port 8082 is the default precisely so that no change is needed.

## What each tab shows

### Acts as itself

The Concierge authenticates with its own credentials and asks for a token bound to the booking API.
No customer is involved, so there is no `act` claim at all.

Watch three things. `sub` is the agent. `aud` is the booking API rather than the agent's own client
id, because the request carries a `resource` (RFC 8707). And `scope` is `booking:read
booking:recommend` and nothing more, because that is exactly what the Concierge's own `Recommender`
role grants. It can recommend a flight; it cannot book one. The remaining highlighted rows are its
identity claims (`name`, `owner`, `ou*`) and its `modelProvider` attribute, opted in through
`token.accessToken.clientConfig.attributes`.

### Acts for a customer

Click **Sign in as John** to start the `authorization_code` flow. The agent is registered with
`pkceRequired: true`, matching what the Console's Delegated mode sets. PKCE (S256) is used on the
authorization request and state is validated on the callback. After logging in at the Gate you land
back on the tab showing:

- **ID token** because `openid` was requested. It identifies the user and never carries `act`.
- **Access token (on-behalf-of)** carrying `sub` (the user), `act` (the agent), `scope`, plus any
  allow-listed user attributes (`email`, `username`) the user actually has. Because the OAuth client
  is an agent, this is an on-behalf-of token by construction, with no extra exchange needed.
- **Refresh token** used by the action below.

Then use the buttons:

- **Refresh access token** calls the `refresh_token` grant. The access token is reissued (watch
  `iat`, `jti`, and `exp` change) and still carries the agent as `act`.
- **Narrow to booking:read for the booking API** calls the token-exchange grant (RFC 8693) with
  John's access token as `subject_token`, the Concierge's own token as `actor_token`, and a
  requested `scope` of `booking:read` against the booking resource. Compare `scope` and `aud` with
  the access token above: `booking:create` is gone. That narrowing is the reason to exchange at all.
  Without it the result would be indistinguishable from its input, since the actor is the same
  agent that was already recorded.

### Delegates to agents

Three exchanges, each narrowed to `booking:read`, showing how ThunderID both nests `act` and shrinks
authority:

| Step | subject_token | actor_token | Resulting token |
|------|---------------|-------------|-----------------|
| Hand off the search | customer | Concierge | `{ sub: customer, act: { sub: Concierge } }` |
| Layered actor | worker | Concierge | `{ sub: worker, act: { sub: Concierge } }` |
| Whole path | customer | layered actor | `{ sub: customer, act: { sub: worker, act: { sub: Concierge } } }` |

The deep `act` is reachable only by feeding an actor token that already carries a nested `act` (the
layered actor step), because each exchange builds `act` solely from the `actor_token` and does not
merge the subject token's existing actor. That is why step 2 exists.

**This tab signs John in itself.** The chain starts with a person asking for something, so step 3
runs its own login at `/chain/callback` and John consents to `openid booking:read booking:create`.
The exchanges then narrow that to `booking:read` for the worker. Nothing is borrowed from the Acts
for a customer tab.

`npm run cli` is the exception: there is no browser, so it uses the Concierge's own token as the
subject and labels it a stand-in. The `act` nesting is identical; only the subject differs.

To see the same chain in the terminal instead:

```bash
npm run cli
```

## Layout

```
server.mjs                             router, tabs, sessions, overview page, the try-it-out steps
config.json                            endpoints, scopes, agent credentials
cli.mjs                                terminal run of the delegation chain
lib/http.mjs                           OAuth token client over Node's built-in http/https
lib/jwt.mjs                            JWT decoding for display, and act-chain flattening
lib/readiness.mjs                      the backend and agent checks behind steps 1 and 2
lib/resources.mjs                      slices the resource file per scenario for download
lib/render.mjs                         the page shell, stylesheet, and shared building blocks
scenarios/self.mjs                     acts as itself
scenarios/obo.mjs                      acts for a user, plus the login and callback routes
scenarios/chain.mjs                    delegates to agents
thunderid-config/thunderid-config.yaml the declarative resources to import into ThunderID
```

A scenario is one module exporting its tab metadata, an `explain` function for the top of the page,
a `render` function for the results, the agents it `requires`, and optionally extra `routes` and a
`check` for prerequisites only it can verify. Adding a fourth posture means adding one file and
listing it in `server.mjs`.

The per-tab downloads are sliced out of the single `thunderid-config/thunderid-config.yaml` at
request time by matching each document's `id`, so there is one source of truth for the agent
definitions and no duplicated YAML.
