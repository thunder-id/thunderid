# App-Native Authentication

By default the Wayfinder sample authenticates users through a browser redirect to the ThunderID-hosted Login Gate. This document describes how to run the sample in **app-native mode**, where authentication happens inside the application itself — no redirect to a hosted page.

Two variants are available:

| Mode | `VITE_AUTH_IS_VERBOSE` | Description |
|---|---|---|
| **Standard** | `false` (default) | The app drives each flow step directly via `/flow/execute` API calls and renders its own step UI. |
| **Verbose** | `true` | The React SDK's `SignIn`/`SignUp` components handle step rendering automatically. |

Both variants cover the same B2C flows: sign-in, registration (with automatic sign-in on completion), and password recovery.

## ThunderID Setup

The app-native config is in `thunderid-config/app-native/`. Use it **instead of** the redirect config.

1. Start ThunderID and open the Console.
2. On the **welcome screen**, choose **Open** and upload `thunderid-config/app-native/thunderid-config.yaml`. For environment variables, upload `thunderid-config/app-native/thunderid.env`.

Key differences from the redirect config:

- Uses `wayfinder-registration-autosignin-flow` — registration completes with an automatic sign-in.
- Password recovery links redirect back to `http://localhost:5173/recovery` (set via `WAYFINDER_RECOVERY_BASE_URL`).
- No AI agent client or CIBA flows — app-native mode is focused on B2C flows only.

### SMTP (for Password Recovery)

Update `deployment.yaml` to deliver recovery emails to the sample inbox:

```yaml
email:
  smtp:
    host: "127.0.0.1"
    port: 2525
    username: "dev"
    password: "dev"
    from_address: "noreply@thunderid.dev"
    enable_start_tls: false
    enable_authentication: true
```

## Configure the Frontend

In `frontend/.env`, set:

```env
VITE_THUNDER_BASE_URL=https://localhost:8090
VITE_THUNDER_APP_ID=wayfinder-app

# Disable redirect-based auth
VITE_AUTH_IS_REDIRECT_BASED=false

# "false" = standard mode (custom step UI), "true" = verbose mode (SDK components)
VITE_AUTH_IS_VERBOSE=false
```

`VITE_THUNDER_CLIENT_ID` is not required in app-native mode.

## Run

```bash
cd backend     && npm install && npm run seed && npm start  # http://localhost:8787
cd smtp-server && npm install && npm run dev                # SMTP :2525 | Inbox http://localhost:8788
cd frontend    && npm install && npm run dev                # http://localhost:5173
```

## Try It

### Sign in

Open `http://localhost:5173` and click **Sign in**. You land on an in-app sign-in page. Enter `john.doe` / `john.doe`.

### Register

Click **Register** on the sign-in page and fill in the form. The registration-with-auto-sign-in flow signs you in immediately on completion.

### Password Recovery

Click **Forgot password**, enter your username, and check the SMTP inbox at `http://localhost:8788` for the recovery email. Follow the link to set a new password.

## Switching Between Modes

Change `VITE_AUTH_IS_VERBOSE` in `frontend/.env` and restart the dev server:

| Value | Behaviour |
|---|---|
| `false` | Standard — each flow step rendered by the app's own UI |
| `true` | Verbose — `SignIn`/`SignUp` components from the React SDK handle step rendering |
