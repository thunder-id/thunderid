# ThunderID React SDK Sample Application

This sample application demonstrates how to integrate authentication into a React application using the `@thunderid/react` SDK. It showcases OAuth 2.0/OIDC based user authentication, token management, and user profile display.

## Features

- 🔐 OAuth 2.0/OIDC authentication
- 👤 Display user profile information (name, username)
- 🎫 View access tokens and decoded JWT components (header, payload, signature)
- 🎨 Modern UI with Oxygen UI components
- 🔄 Token refresh and session management
- 📱 Responsive design

## Prerequisites

- Node.js 20+
- A running server instance (default: `https://localhost:8090`)
- An OAuth application registered in with appropriate redirect URIs

## Quick Start (Pre-Built Application)

If you have the pre-built distribution, you can run it directly:

### 1. Import ThunderID Resources

The sample ships with a `thunderid-config/` directory containing a declarative YAML file that creates the required user type and application (referencing the default OU by handle) in one step.

1. Open `thunderid-config/thunderid.env` and set your preferred credentials:

    ```bash
    REACT_SDK_SAMPLE_CLIENT_ID=REACT_SDK_SAMPLE
    REACT_SDK_SAMPLE_REDIRECT_URIS=["https://localhost:3000"]
    ```

2. Import via the ThunderID Console ([https://localhost:8090/console](https://localhost:8090/console)):
   - **First-time login**: a welcome screen appears with an **Open** button to upload the YAML file directly.
   - **Later**: access the same welcome screen from the user profile menu in the top-right corner of the console.

This creates the `Customer` user type and the `React SDK Sample` application under the default organization unit.

### 2. Configure the Application

Open `dist/runtime.json` and set the `clientId` to the value you used in `thunderid-config/thunderid.env`:

```json
{
  "clientId": "{your-client-id}",
  "baseUrl": "https://localhost:8090",
  "scopes": ["openid", "profile"]
}
```

| Property | Description |
|----------|-------------|
| `clientId` | The OAuth client ID configured in `thunderid.env` |
| `baseUrl` | The base URL of your server |
| `scopes` | Optional OAuth scopes as a string array or a space/comma-delimited string |

### 3. Start the Application

```bash
npm install
npm start
```

### 4. Access the Application

Open your browser and navigate to [https://localhost:3000](https://localhost:3000) (or `http://localhost:3000` if running without SSL)

## Development

To run the application in development mode with hot reloading:

### 1. Install Dependencies

```bash
npm install
```

### 2. Set Up SSL Certificates

For HTTPS support during development, copy the SSL certificates from your distribution to the project root:

```bash
# From distribution
cp /path/to/thunder/repository/resources/security/server.key .
cp /path/to/thunder/repository/resources/security/server.cert .

# Or from build output (if building from source)
cp ../../target/out/.cert/server.key .
cp ../../target/out/.cert/server.cert .
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
| `npm run build` | Build for production (outputs to `dist/`) |
| `npm run preview` | Preview the production build locally |
| `npm run lint` | Run ESLint to check code quality |

## Configuration Reference

### Complete `runtime.json` Schema

```json
{
  "clientId": "string (required) - OAuth client ID",
  "baseUrl": "string (required) - Server base URL",
  "scopes": "string | string[] (optional) - Requested OAuth scopes"
}
```

### Environment Variable Fallbacks

If a value is missing from `runtime.json`, the app falls back to Vite environment variables:

- `VITE_REACT_APP_CLIENT_ID`
- `VITE_THUNDERID_BASE_URL`
- `VITE_REACT_APP_SCOPES` (optional, space/comma-delimited list, e.g. `openid profile`)

### Application Setup

Before running the app, ensure your application is configured with:

1. **Authorized Redirect URLs**: Add your application URL (e.g., `https://localhost:3000`)
2. **Allowed Origins**: Add your application origin for CORS
3. **Grant Types**: Authorization Code (with PKCE required for SPAs)

## Troubleshooting

### Common Issues

**Issue**: "Failed to fetch token"
- Ensure server is running and accessible at the configured base URL
- Verify the client ID is correct
- Check that redirect URLs are properly configured in the server

**Issue**: "Invalid client" error
- Double-check the `clientId` in your `runtime.json`
- Ensure the application exists in and is enabled

**Issue**: CORS errors
- Add your application URL to "Allowed Origins" in  `deployment.yaml`:
  ```yaml
  cors:
    allowed_origins:
      - "https://localhost:3000"
  ```

## How It Works

### Authentication Flow

1. **SDK Provider Setup**: The app wraps components with `AsgardeoProvider` configured with base URL and client ID
2. **Conditional Rendering**: Uses `SignedIn`/`SignedOut` components to show appropriate content based on auth state
3. **Token Management**: Retrieves and decodes JWT tokens to display user information

### Key Code Examples

**Provider Configuration:**
```tsx
<AsgardeoProvider
  baseUrl={config.baseUrl}
  clientId={config.clientId}
  platform="AsgardeoV2"
>
  <App />
</AsgardeoProvider>
```

**Using Authentication Hooks:**
```tsx
import { useThunderID } from "@thunderid/react";

const { getAccessToken, signIn } = useThunderID();
const accessToken = await getAccessToken();
```

## License

Licensed under the Apache License, Version 2.0. You may not use this file except in compliance with the License.

---------------------------------------------------------------------------
(c) Copyright 2025 WSO2 LLC.

