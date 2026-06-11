# React Vanilla Sample Application

A sample React application that demonstrates app-native flow orchestration with ThunderID — covering login, registration, and basic profile management.

## Prerequisites

- Node.js 20+
- A running ThunderID server (default: `https://localhost:8090`)

## Quick Start

### 1. Pick a Scenario

Two ready-to-use configurations are provided under `thunderid-config/`:

| Folder | What it sets up |
|--------|-----------------|
| `basic/` | Username and password login — simplest way to get started |
| `multi-auth/` | Username/password + Google, GitHub, SMS OTP, and Passkey |

### 2. Import ThunderID Resources

Open the `.env` file in your chosen folder and fill in your values:

**`basic/thunderid.env`** — only two values needed:
```
SAMPLE_APP_CLIENT_ID=sample_app_client
SAMPLE_APP_REDIRECT_URIS=["https://localhost:3000"]
```

**`multi-auth/thunderid.env`** — also requires social IdP and SMS credentials:
```
SAMPLE_APP_GOOGLE_CLIENT_ID=
SAMPLE_APP_GOOGLE_CLIENT_SECRET=
SAMPLE_APP_GITHUB_CLIENT_ID=
SAMPLE_APP_GITHUB_CLIENT_SECRET=
SAMPLE_APP_SMS_SENDER_ID=
```

Then import via the ThunderID Console (`https://localhost:8090/console`):
- **First time**: a welcome screen appears with an **Open** button to upload the YAML.
- **Later**: access the same screen from the user profile menu (top-right).

This creates the `Customer` user type and `Sample App` application (ID: `019e3a5c-0500-7f3e-a66e-66fc7918c3a7`) in the default organization unit.

### 3. Configure the Application

Open `public/runtime.json` and set the application ID:

```json
{
    "flowEndpoint": "https://localhost:8090/flow",
    "applicationID": "019e3a5c-0500-7f3e-a66e-66fc7918c3a7"
}
```

### 4. Start the Application

```bash
npm install
npm start
```

> SSL certificates are required. Copy `server.key` and `server.cert` from your ThunderID distribution, or generate self-signed ones:
> ```bash
> openssl req -nodes -new -x509 -keyout server.key -out server.cert
> ```

### 5. Open the App

[https://localhost:3000](https://localhost:3000)

## Further Reading

See [docs/REFERENCE.md](docs/REFERENCE.md) for:
- Detailed config reference (`runtime.json`, Passkey, Invite flows)
- UI rendering and action `ref` conventions
- Hosting options and available scripts

## License

Licensed under the Apache License, Version 2.0.

---------------------------------------------------------------------------
(c) Copyright 2025 WSO2 LLC.
