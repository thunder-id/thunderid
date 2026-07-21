# React Vanilla Sample — Detailed Reference

This document covers advanced configuration, flow details, and developer options for the React Vanilla Sample Application. For getting started quickly, see the [main README](../README.md).

## ThunderID Configuration Scenarios

Two configuration sets are available under `thunderid-config/`. Each contains a `thunderid-config.yaml` (declarative resources) and a `thunderid.env` (environment values for the YAML templates).

### `basic/`

Sets up:
- `Customer` user type (username, password, email, name fields)
- `Sample App` application using the built-in `default-flow`
- Registration enabled, recovery disabled

Required environment values:

| Variable | Description |
|----------|-------------|
| `SAMPLE_APP_CLIENT_ID` | OAuth2 client ID for the application |
| `SAMPLE_APP_REDIRECT_URIS` | JSON array of allowed redirect URIs |

### `multi-auth/`

Sets up everything in `basic/`, plus:
- Google OIDC identity provider
- GitHub OAuth identity provider
- Custom multi-step authentication flows (single-step and dual-step with SMS OTP)
- Registration flows with Google, GitHub, SMS OTP, and Passkey options

Required environment values:

| Variable | Description |
|----------|-------------|
| `SAMPLE_APP_CLIENT_ID` | OAuth2 client ID for the application |
| `SAMPLE_APP_REDIRECT_URIS` | JSON array of allowed redirect URIs |
| `SAMPLE_APP_GOOGLE_CLIENT_ID` | Google OAuth app client ID |
| `SAMPLE_APP_GOOGLE_CLIENT_SECRET` | Google OAuth app client secret |
| `SAMPLE_APP_GOOGLE_REDIRECT_URI` | Redirect URI registered in Google (e.g. `https://localhost:3000/`) |
| `SAMPLE_APP_GOOGLE_SCOPES` | Scopes to request (e.g. `openid,email,profile`) |
| `SAMPLE_APP_GITHUB_CLIENT_ID` | GitHub OAuth app client ID |
| `SAMPLE_APP_GITHUB_CLIENT_SECRET` | GitHub OAuth app client secret |
| `SAMPLE_APP_GITHUB_REDIRECT_URI` | Redirect URI registered in GitHub |
| `SAMPLE_APP_SMS_SENDER_ID` | ID of an existing SMS notification sender in ThunderID |

> `SAMPLE_APP_SMS_SENDER_ID` must reference a sender already configured in the server — the YAML does not create it.

## Supported Authentication Methods

| Method | Description |
|--------|-------------|
| Basic authentication | Username and password via `CredentialsAuthExecutor` |
| Google | Social login via `GoogleOIDCAuthExecutor` |
| GitHub | Social login via `GithubOAuthExecutor` |
| SMS OTP | One-time password via `OTPExecutor` (generate + verify modes) paired with `SMSExecutor` for delivery |
| Passkeys | FIDO2/WebAuthn via `PasskeyAuthExecutor` (challenge + verify / register modes) |

The UI adapts automatically to the options returned by the active flow — no code changes needed when switching flows.

## Runtime Configuration

`public/runtime.json` is loaded at startup and controls which server and application the sample connects to:

```json
{
    "flowEndpoint": "https://localhost:8090/flow",
    "applicationID": "{your-application-id}"
}
```

| Property | Description |
|----------|-------------|
| `flowEndpoint` | Flow orchestration API endpoint |
| `applicationID` | The application ID registered in ThunderID |

## Passkey Configuration

WebAuthn requires the server to validate that the credential was created from a trusted origin. By default, ThunderID only allows `https://localhost:8090`. When running the sample at `https://localhost:3000`, add it to the allowed origins in the server's `deployment.yaml`:

```yaml
passkey:
  allowed_origins:
    - "https://localhost:8090"
    - "https://localhost:3000"
```

Without this, passkey registration fails with an origin validation error.

## Invite Flow Configuration

For invite-based flows (e.g. password recovery), set `inviteBaseURL` on the `InviteExecutor` node (in `generate` mode) in the flow definition. Point it to the sample's invite page:

```
https://localhost:3000/invite
```

Without this, generated invite links fall back to the server's default Gate URL.

## UI Rendering and Action Ref Convention

Action button appearance is driven by the `ref` value of each action node. The following keywords trigger special rendering:

| Keyword in `ref` | Rendered as |
|------------------|-------------|
| `basic_auth` | Username and password form |
| `google` | "Continue with Google" with Google icon |
| `github` | "Continue with GitHub" with GitHub icon |
| `sms` or `mobile` | "Continue with SMS OTP" with SMS icon |
| `passkey` | "Sign in with Passkey" with fingerprint icon |
| `signin` or `sign_in` | "Sign In" submit button |
| `signup` or `sign_up` | "Create Account" submit button |

Any other `ref` value is rendered as a plain button using the `ref` text as the label.

## Available Scripts

| Command | Description |
|---------|-------------|
| `npm run dev` | Start development server with hot reloading |
| `npm run build` | Build for production (outputs to `dist/`) |
| `npm run preview` | Preview the production build locally |
| `npm run lint` | Run ESLint |
| `npm start` | Build and start the production server |

## Hosting Options

### Using the Provided Node Server

```bash
npm install && npm run build
cd server && npm start
```

### Using Your Own Web Server

Host the contents of `dist/` on any HTTPS-capable web server. Ensure `runtime.json` is served and accessible at runtime.
