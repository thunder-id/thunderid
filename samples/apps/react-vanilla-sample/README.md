# React Vanilla Sample Application

This sample React application demonstrates integrating authentication and registration into your application using app-native flow orchestration API.

## Prerequisites

- Node.js 20+
- A running server instance (default: `https://localhost:8090`)
- An application registered in the server

## Quick Start (Pre-built Application)

If you have the pre-built distribution, you can run it directly:

### 1. Configure the Application

Open `app/runtime.json` and configure the settings:

```json
{
    "flowEndpoint": "https://localhost:8090/flow",
    "applicationsEndpoint": "https://localhost:8090/applications",
    "applicationID": "{your-application-id}"
}
```

| Property | Description |
|----------|-------------|
| `flowEndpoint` | Flow orchestration endpoint |
| `applicationsEndpoint` | Applications API endpoint |
| `applicationID` | The application ID registered in the server (obtained during server setup) |

#### Expected Flow Node IDs

When using Native Flow, the sample app UI renders based on `nextNode` values in the flow definition. Your flow should use these node IDs for proper UI rendering:

| Node ID | Purpose |
|---------|---------|
| `basic_auth` | Username/password authentication |
| `github_auth` | GitHub OAuth |
| `google_auth` | Google OAuth |
| `prompt_mobile` or `mobile_prompt_username` | SMS OTP authentication |

## Setting Up the Registration Flow

This sample supports a multi-auth registration flow that includes username/password, Google OAuth, GitHub OAuth, and SMS OTP. You can create this flow from the Thunder Console.

### 1. Create the Registration Flow

1. Open the Thunder Console at `https://localhost:8090` and sign in.
2. Navigate to **Flows** and click **Create Flow**.
3. Select the **Registration** flow type.
4. Choose the **Basic + Google + GitHub + SMS** template.
5. Note the **Flow Handle** (e.g., `fancy-cities-walk`) — the sample app uses this handle to start registration.

### 2. Assign the Flow to Your Application

1. In the Thunder Console, navigate to **Applications** and open your application.
2. Under **Flows**, set the **Registration Flow** to the flow you just created.
3. Save the changes.

### 3. Enable Self-Registration

For users to be able to register, self-registration must be enabled in your ThunderID instance and the application must be configured to allow it:

1. In the Thunder Console, navigate to **Settings > User Registration** and ensure **Allow Self Registration** is enabled.
2. Open your application and check that the **User Type** assigned to self-registered users has registration permissions enabled.

## Setting Up Social Login Providers

To enable Google and GitHub login, configure OAuth clients in those providers and register them as identity providers in ThunderID.

### Google OAuth

#### Step 1 — Create a Google OAuth Client

1. Go to the [Google Cloud Console](https://console.cloud.google.com/) and open your project (or create one).
2. Navigate to **APIs & Services > Credentials** and click **Create Credentials > OAuth client ID**.
3. Select **Web application** as the application type.
4. Under **Authorized redirect URIs**, add `https://localhost:3000/`.
5. Click **Create** and note the **Client ID** and **Client Secret**.

#### Step 2 — Configure the Google IdP in ThunderID

Use the ThunderID management API to update the Google IdP with your credentials:

```bash
# Get an admin token first (replace with your admin credentials)
TOKEN=$(curl -sk -X POST 'https://localhost:8090/flow/execute' \
  -H 'Content-Type: application/json' \
  -d '{"applicationId":"<admin-app-id>","flowType":"AUTHENTICATION"}' | \
  # ... complete the auth flow to obtain an assertion token
  )

# Find the Google IDP ID
curl -sk -X GET 'https://localhost:8090/identity-providers' \
  -H "Authorization: Bearer $TOKEN"

# Update the Google IDP
curl -sk -X PUT 'https://localhost:8090/identity-providers/<google-idp-id>' \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "clientId": "<your-google-client-id>",
    "clientSecret": "<your-google-client-secret>",
    "redirectUri": "https://localhost:3000/"
  }'
```

### GitHub OAuth

#### Step 1 — Create a GitHub OAuth App

1. Go to **GitHub > Settings > Developer settings > OAuth Apps** and click **New OAuth App**.
2. Set the **Homepage URL** to `https://localhost:3000`.
3. Set the **Authorization callback URL** to `https://localhost:3000/`.
4. Click **Register application** and note the **Client ID**.
5. Click **Generate a new client secret** and note the **Client Secret**.

#### Step 2 — Configure the GitHub IdP in ThunderID

```bash
# Find the GitHub IDP ID
curl -sk -X GET 'https://localhost:8090/identity-providers' \
  -H "Authorization: Bearer $TOKEN"

# Update the GitHub IDP
curl -sk -X PUT 'https://localhost:8090/identity-providers/<github-idp-id>' \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "clientId": "<your-github-client-id>",
    "clientSecret": "<your-github-client-secret>",
    "redirectUri": "https://localhost:3000/"
  }'
```

## Setting up SMS OTP

SMS OTP requires a notification sender configured in ThunderID. ThunderID supports any SMS provider that accepts HTTP requests. Point it at your preferred SMS gateway or service.

### Step 1 — Create a Notification Sender

```bash
curl -sk -X POST 'https://localhost:8090/notification-senders/message' \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "My SMS Sender",
    "description": "SMS sender for OTP delivery",
    "provider": "custom",
    "properties": [
      {"name": "url",          "value": "<your-sms-provider-webhook-url>", "is_secret": false},
      {"name": "http_method",  "value": "POST",   "is_secret": false},
      {"name": "content_type", "value": "JSON",   "is_secret": false}
    ]
  }'
```

Note the `id` field from the response — this is your **Sender ID**.

### Step 2 — Update the Registration Flow with the Sender ID

After creating the notification sender, update the `send_sms` and `verify_sms` nodes in your registration flow to use the sender ID:

```bash
# Get the flow definition
curl -sk -X GET 'https://localhost:8090/flows/<flow-id>' \
  -H "Authorization: Bearer $TOKEN"

# Update the flow — set the senderId in both the send_sms and verify_sms
# nodes' properties to your Sender ID, then PUT the updated flow:
curl -sk -X PUT 'https://localhost:8090/flows/<flow-id>' \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" \
  -d '<updated-flow-json>'
```

### 2. Start the Application

**Linux/macOS:**
```bash
sh start.sh
```

**Windows:**
```powershell
.\start.ps1
```

### 3. Access the Application

Open your browser and navigate to [https://localhost:3000](https://localhost:3000)

## Development

To run the application in development mode with hot reloading:

### 1. Install Dependencies

```bash
npm install
```

### 2. Set Up SSL Certificates

For HTTPS support, copy the SSL certificates from your server distribution to the project root:

```bash
# From distribution
cp /path/to/thunder/repository/resources/security/server.key .
cp /path/to/thunder/repository/resources/security/server.cert .

# Or from build output (if building from source)
cp ../../target/out/.cert/server.key .
cp ../../target/out/.cert/server.cert .
```

Or generate self-signed certificates:

```bash
openssl req -nodes -new -x509 -keyout server.key -out server.cert
```

### 3. Start Development Server

```bash
npm run dev
```

The application will be available at [https://localhost:3000](https://localhost:3000)

### Available Scripts

| Command | Description |
|---------|-------------|
| `npm run dev` | Start development server with hot reloading |
| `npm run build` | Build for production (outputs to `dist/` and prepares server) |
| `npm run preview` | Preview the production build locally |
| `npm run lint` | Run ESLint to check code quality |
| `npm start` | Build and preview the production application |

## Hosting Options

This sample includes a pre-built application with a simple Node.js server. You can also host the application on your own web server.

### Using the Provided Node Server

The sample comes with a built-in Node.js server that serves the React app over HTTPS.

1. Install dependencies and build:
   ```bash
   npm install
   npm run build
   ```

2. Start the server:
   ```bash
   cd server
   npm start
   ```

### Using Your Own Web Server

The `app` folder (or `dist` after building) contains the built application that can be hosted on any web server. Configure your server to:

1. Serve the static files from the `app` or `dist` folder
2. Set up HTTPS with valid certificates
3. Ensure `runtime.json` is accessible and editable for configuration

## License

Licensed under the Apache License, Version 2.0. You may not use this file except in compliance with the License.

---------------------------------------------------------------------------
(c) Copyright 2025 WSO2 LLC.
