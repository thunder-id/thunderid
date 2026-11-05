# Vanilla Sample Application

A sample Next.js application that demonstrates app-native flow orchestration with ThunderID, covering login, registration, and basic profile management, using a backend-for-frontend (BFF) architecture.

App-native sign-in requires the initiating client to authenticate itself to the Flow Execution API with a **Flow Secret**. Browser single-page applications cannot hold that secret safely, so this sample keeps it server-side: every call to `POST /flow/execute` is proxied through a Next.js server route ([`src/app/api/flow/route.ts`](src/app/api/flow/route.ts)), which attaches the secret and stores the resulting auth assertion in an httpOnly cookie. The browser never sees the secret or the assertion; see [docs/REFERENCE.md](docs/REFERENCE.md) for the full architecture.

## Prerequisites

- Node.js 20.9+
- A running ThunderID server (default: `https://localhost:8090`)

## Quick Start

### 1. Pick a Scenario

Two ready-to-use configurations are provided under `thunderid-config/`:

| Folder | What it sets up |
|--------|-----------------|
| `basic/` | Username and password login, the simplest way to get started |
| `multi-auth/` | Username/password + Google, GitHub, SMS OTP, and Passkey |

### 2. Import ThunderID Resources

Open the `.env` file in your chosen folder and fill in your values:

**`basic/thunderid.env`** only needs one value:
```dotenv
SAMPLE_APP_FLOW_SECRET=sample-app-flow-secret
```

**`multi-auth/thunderid.env`** also requires social IdP and SMS credentials:
```dotenv
SAMPLE_APP_FLOW_SECRET=sample-app-flow-secret
SAMPLE_APP_GOOGLE_CLIENT_ID=
SAMPLE_APP_GOOGLE_CLIENT_SECRET=
SAMPLE_APP_GOOGLE_REDIRECT_URI=https://localhost:3000/
SAMPLE_APP_GITHUB_CLIENT_ID=
SAMPLE_APP_GITHUB_CLIENT_SECRET=
SAMPLE_APP_GITHUB_REDIRECT_URI=https://localhost:3000/
SAMPLE_APP_SMS_SENDER_ID=
```

`SAMPLE_APP_SMS_SENDER_ID` isn't a value you choose; it's the ID of an SMS sender you create on the server first. See [docs/REFERENCE.md](docs/REFERENCE.md#multi-auth) for how to create one, including a no-real-SMS-provider option for local testing.

Then import via the ThunderID Console (`https://localhost:8090/console`):
- **First time**: a welcome screen appears with an **Open** button to upload the YAML.
- **Later**: access the same screen from the user profile menu (top-right).

This creates the `Customer` user type and the `Sample App` application (ID: `019e3a5c-0500-7f3e-a66e-66fc7918c3a7`) in the system organization unit. Note the value you set for `SAMPLE_APP_FLOW_SECRET`; you'll need it in the next step.

### 3. Configure the Application

Copy `.env.example` to `.env.local` and fill in the application ID and the Flow Secret you set above:

```bash
cp .env.example .env.local
```

```dotenv
THUNDERID_SERVER_URL=https://localhost:8090
THUNDERID_APPLICATION_ID=019e3a5c-0500-7f3e-a66e-66fc7918c3a7
THUNDERID_FLOW_SECRET=sample-app-flow-secret
```

These are only read on the server (see [`src/lib/server/thunderid.ts`](src/lib/server/thunderid.ts)) and are never bundled into client-side JavaScript.

If you got this sample from a packaged ThunderID distribution, it already trusts that
distribution's own server certificate out of the box (bundled at `certificates/server.cert`) and
you can skip this. If you're running from a monorepo checkout instead, also set
`THUNDERID_SERVER_CA_CERT` to your server's certificate. This is required by default: both
`make run_backend` and a server built from a distribution use a self-signed certificate unless
you've configured one from a trusted CA.

```dotenv
THUNDERID_SERVER_CA_CERT=<repo-root>/backend/cmd/server/config/certs/server.cert
```

Flow calls happen server-side, so Node's `fetch()` needs to trust the certificate itself; a
browser's "proceed anyway" click doesn't help here. (Setting `NODE_EXTRA_CA_CERTS` instead does
**not** work here: `next dev --experimental-https` overwrites it with its own certificate path; see
[vercel/next.js#57958](https://github.com/vercel/next.js/issues/57958).) Without this, requests
from `/api/flow` fail with `DEPTH_ZERO_SELF_SIGNED_CERT` and the browser shows "A network error
occurred while contacting the ThunderID server."

### 4. Start the Application

```bash
npm install
npm run dev
```

`next dev --experimental-https` generates and trusts a local certificate on first run, which may prompt for your password to install the local certificate authority. If that install can't complete (for example, no sudo access), the dev server falls back to plain HTTP; passkeys and some social login callbacks require HTTPS, so grant the prompt if you plan to try those.

### 5. Open the App

[https://localhost:3000](https://localhost:3000)

## Further Reading

See [docs/REFERENCE.md](docs/REFERENCE.md) for:
- The BFF architecture and where the Flow Secret and auth assertion live
- Detailed config reference (env vars, Passkey, Invite flows)
- UI rendering and action `ref` conventions
- Available scripts

## License

Licensed under the Apache License, Version 2.0. You may not use this file except in compliance with the License.
