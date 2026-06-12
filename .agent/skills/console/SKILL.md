---
name: console
description: Navigate and interact with the ThunderID Console UI. Use when exploring the ThunderID admin console, testing UI changes, creating users/applications/roles, or debugging the frontend.
allowed-tools: Bash(playwright-cli:*) Bash(npx:*)
---

# ThunderID Console Navigation with playwright-cli

## Resolving the Console Base URL

Before running any commands, determine the console base URL. Do NOT hardcode a URL — resolve it from project configuration:

1. **Check `deployment.yaml`** at `backend/cmd/server/deployment.yaml` for the `server.hostname` and `server.port`. If the backend is serving the console (production mode), the URL is `https://{hostname}:{port}/console`.
2. **Check `vite.config.ts`** at `frontend/apps/console/vite.config.ts` for the development server `PORT` (default `5191`) and `HOST` (default `localhost`). If the frontend development server is running separately, the URL is `https://{HOST}:{PORT}/console`.
3. **Check environment variables**: `PORT`, `HOST`, or `BASE_URL` may override the defaults.
4. **If unable to resolve**, ask the user for the ThunderID Console URL.

Use the resolved URL as `{CONSOLE_URL}` in all commands below (e.g., `{CONSOLE_URL}`).

## Quick Start

```bash
# Open Console (redirects to sign-in gate)
playwright-cli open {CONSOLE_URL} -s=thunderid

# After authenticating (see below), navigate directly
playwright-cli goto {CONSOLE_URL}/users -s=thunderid

# Snapshot the page to see element refs
playwright-cli snapshot -s=thunderid

# Interact with elements using refs from snapshot
playwright-cli click e15 -s=thunderid

# Take a screenshot
playwright-cli screenshot -s=thunderid

# Close the browser
playwright-cli close -s=thunderid
```

## Prerequisites

If `playwright-cli` is not installed:

```bash
npm install -g @playwright/cli@latest
```

All commands use the named session `-s=thunderid` so the browser persists across commands.

## Authentication

ThunderID Console requires authentication. The sign-in form is dynamically rendered by the ThunderID SDK, so always use `snapshot` to get element refs before interacting.

Default credentials: `admin` / `admin`

### First-Time Login

```bash
# 0. Accept self-signed certs first (see Troubleshooting for details)
#    Open blank session, navigate to each origin, click through cert warnings
playwright-cli open -s=thunderid
# Accept backend cert, then console cert, then gate cert (see Troubleshooting)

# 1. Navigate to the console (auto-redirects to /gate/signin)
playwright-cli goto {CONSOLE_URL} -s=thunderid

# 2. Snapshot to see the login form elements
playwright-cli snapshot -s=thunderid

# 3. Fill username (use the ref from snapshot for the username input)
playwright-cli fill <username-ref> "admin" -s=thunderid

# 4. Fill password (use the ref from snapshot for the password input)
playwright-cli fill <password-ref> "admin" -s=thunderid

# 5. Click Sign In (use the ref from snapshot for the submit button)
playwright-cli click <submit-ref> -s=thunderid

# 6. Verify redirect to console home
playwright-cli snapshot -s=thunderid

# 7. Save auth state for reuse
playwright-cli state-save thunderid-auth -s=thunderid
```

### Reuse Saved Auth

```bash
playwright-cli open -s=thunderid
playwright-cli state-load thunderid-auth -s=thunderid
playwright-cli goto {CONSOLE_URL} -s=thunderid
```

## Route Map

The ThunderID Console base path is `/console`. Append routes below to the resolved `{CONSOLE_URL}`. The sidebar is organized into categories.

### Sidebar Navigation

| Category | Page | Path |
|---|---|---|
| - | Home | `/console/home` |
| Resources | Applications | `/console/applications` |
| Identities | Users | `/console/users` |
| Identities | Groups | `/console/groups` |
| Identities | Roles | `/console/roles` |
| Identities | User Types | `/console/user-types` |
| Configure | Organization Units | `/console/organization-units` |
| Configure | Flows | `/console/flows` |
| Configure | Integrations | `/console/integrations` |
| Customize | Design | `/console/design` |
| Customize | Translations | `/console/translations` |

### Creation Routes

| Resource | Path |
|---|---|
| Application | `/console/applications/create` |
| User | `/console/users/create` |
| Invite User | `/console/users/invite` |
| Group | `/console/groups/create` |
| Role | `/console/roles/create` |
| User Type | `/console/user-types/create` |
| Organization Unit | `/console/organization-units/create` |
| Theme | `/console/design/themes/create` |
| Translation | `/console/translations/create` |

### Detail/Edit Routes

| Resource | Path |
|---|---|
| Application | `/console/applications/:applicationId` |
| User | `/console/users/:userId` |
| Group | `/console/groups/:groupId` |
| Role | `/console/roles/:roleId` |
| User Type | `/console/user-types/:id` |
| Organization Unit | `/console/organization-units/:id` |
| Theme Builder | `/console/design/themes/:themeId` |
| Layout Builder | `/console/design/layouts/:layoutId` |
| Login Flow | `/console/flows/signin` or `/console/flows/signin/:flowId` |
| Translation | `/console/translations/:language` |

## Common Patterns

### Navigate to a Page

```bash
playwright-cli goto {CONSOLE_URL}/users -s=thunderid
playwright-cli snapshot -s=thunderid
```

### Navigate via Sidebar

```bash
# Snapshot to see sidebar element refs
playwright-cli snapshot -s=thunderid
# Click a sidebar item by its ref
playwright-cli click <sidebar-item-ref> -s=thunderid
```

### Create a Resource (e.g., User)

```bash
# Navigate to users list
playwright-cli goto {CONSOLE_URL}/users -s=thunderid
playwright-cli snapshot -s=thunderid

# Click the "Add User" or create button
playwright-cli click <create-button-ref> -s=thunderid
playwright-cli snapshot -s=thunderid

# Fill the creation form fields using refs from snapshot
playwright-cli fill <field-ref> "value" -s=thunderid

# Submit
playwright-cli click <submit-ref> -s=thunderid
playwright-cli snapshot -s=thunderid
```

### Search in a List

```bash
playwright-cli goto {CONSOLE_URL}/users -s=thunderid
playwright-cli snapshot -s=thunderid
playwright-cli fill <search-input-ref> "john" -s=thunderid
playwright-cli snapshot -s=thunderid
```

### Inspect an Element

```bash
playwright-cli eval "el => el.getAttribute('data-testid')" <ref> -s=thunderid
playwright-cli eval "el => el.textContent" <ref> -s=thunderid
```

### Take a Screenshot

```bash
playwright-cli screenshot -s=thunderid
playwright-cli screenshot --filename=console-users.png -s=thunderid
```

## Troubleshooting

- **Redirected to `/gate/signin`**: Auth expired. Re-authenticate or run `playwright-cli state-load thunderid-auth -s=thunderid`.
- **Elements not found in snapshot**: Page may still be loading. Wait a moment and run `playwright-cli snapshot -s=thunderid` again.
- **HTTPS certificate errors**: ThunderID uses self-signed certificates on multiple origins (gate, console, backend). The browser will block navigation with `ERR_CERT_AUTHORITY_INVALID`. To bypass, open a blank session first, then navigate via JS `eval` to trigger Chrome's interstitial error page, and click through it:

  ```bash
  # 1. Open a blank browser session
  playwright-cli open -s=thunderid

  # 2. Navigate to the target URL (triggers cert error page)
  playwright-cli eval "window.location.assign('{CONSOLE_URL}')" -s=thunderid

  # 3. Click through the cert warning
  playwright-cli snapshot -s=thunderid          # find the "Advanced" button ref
  playwright-cli click <advanced-ref> -s=thunderid
  playwright-cli snapshot -s=thunderid          # find the "Proceed to localhost (unsafe)" link ref
  playwright-cli click <proceed-ref> -s=thunderid
  ```

  **Important**: You must accept certs for **each origin** the console talks to. The console redirects to the gate for auth, which calls the backend. If the backend cert is not accepted in the same browser session, API calls will silently fail. Resolve the backend port from `deployment.yaml` (`server.port`) and the gate port from the console's runtime config. Accept certs for each origin before proceeding.

- **Login form not visible**: The ThunderID SDK renders the form dynamically. Take a snapshot after a brief wait. If you see a loading spinner, snapshot again after a few seconds.
- **Session lost**: Run `playwright-cli list` to check active sessions. Start a new one with `playwright-cli open -s=thunderid`.
