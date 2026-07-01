# ThunderID API-Based Authentication Sample Application

This sample application demonstrates how to integrate authentication into a React application using direct API calls instead of SDK-based OAuth redirects. It showcases API-based user registration (sign-up) and authentication (sign-in) flows.

## Features

- User registration via User Management API
- User authentication via Credentials Authentication API
- JWT assertion token handling and display
- Dashboard with user list view
- User profile modal with decoded token information


## Prerequisites

- Node.js 20+
- A running server instance (default: `https://localhost:8090`)
- Server configured with appropriate CORS settings
- SSL certificates (`server.key` and `server.cert`) in the project root
- The "Customer" user type created in ThunderID

## Quick Start

### 1. Configure the Application

Edit `public/config.json` with your server settings:

```json
{
  "baseUrl": "https://localhost:8090"
}
```

| Property | Description |
|----------|-------------|
| `baseUrl` | The base URL of your server |

### 2. Set Up SSL Certificates

The application runs on HTTPS. Copy the SSL certificates from your distribution:

```bash
# From distribution
cp /path/to/thunder/config/certs/server.key .
cp /path/to/thunder/config/certs/server.cert .

# Or from build output (if building from source)
cp ../../target/out/.cert/server.key .
cp ../../target/out/.cert/server.cert .
```

### 3. Set Up Sample Resources

The sample ships with a `thunderid-config/` directory containing the `Customer` user type definition required for sign-up and sign-in.

Import `thunderid-config/thunderid-config.yaml` via the ThunderID Console ([https://localhost:8090/console](https://localhost:8090/console)):
- **First-time login**: a welcome screen appears with an **Open** button to upload the YAML file directly.
- **Later**: access the same welcome screen from the user profile menu in the top-right corner of the console.

### 4. Install Dependencies

```bash
npm install
```

### 5. Start the Development Server

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

## Important: Sign Up Requirements

To use the sign-up functionality, you need to temporarily disable security by setting the following environment variable before starting the server:

```bash
export SKIP_SECURITY=true
```

This is required because the sign-up API creates users without authentication. In a production environment, you would typically use a different approach such as:
- OAuth-based registration flows
- Admin-created user accounts
- Custom registration endpoints with appropriate security controls

## Application Structure

```
src/
├── components/
│   ├── Layout.tsx           # Main layout with navigation
│   ├── ThemeSwitcher.tsx    # Dark/light theme toggle
│   ├── UserProfileModal.tsx # User profile display dialog
│   └── UserTable.tsx        # Registered users table
├── pages/
│   ├── HomePage.tsx         # Landing page with sign-in/sign-up options
│   ├── SignInPage.tsx       # User authentication form
│   ├── SignUpPage.tsx       # User registration form
│   └── DashboardPage.tsx    # Authenticated user dashboard
├── utils/
│   ├── api.ts               # API utilities
│   └── jwt.ts               # JWT decoding utilities
├── config.ts                # Runtime configuration loader
├── router.tsx               # Application routes
└── main.tsx                 # Application entry point
```

## API Endpoints Used

This sample interacts with the following APIs:

### Authentication
- `POST /auth/credentials/authenticate` - Authenticate user with username/password

### User Management
- `GET /users` - List registered users
- `POST /users` - Register a new user

### Organization Units
- `GET /organization-units` - Get available organization units

## How It Works

### Sign Up Flow
1. User fills in the registration form (username, name, email, password)
2. Application fetches the default organization unit ID
3. Sends a POST request to `/users` with user attributes and type "Customer"
4. On success, displays confirmation message

### Sign in Flow
1. User enters username and password
2. Application sends credentials to `/auth/credentials/authenticate`
3. On success, receives an assertion token (JWT)
4. Token is stored in `sessionStorage`
5. User is redirected to the dashboard

### Dashboard
- Displays the authenticated user's information
- Shows a table of all registered users
- Allows viewing detailed profile information in a modal

## Troubleshooting

### Common Issues

**Issue**: "Failed to fetch" errors
- Ensure server is running and accessible at the configured base URL
- Check the CORS configuration in the server-config `cors` section

**Issue**: "User type not found" error during sign-up
- Import `thunderid-config/thunderid-config.yaml` via the ThunderID Console (see "Set Up Sample Resources" above) to create the "Customer" user type

**Issue**: Sign-up fails with authentication/authorization errors
- Ensure `SKIP_SECURITY=true` is set when starting the server

**Issue**: CORS errors
- Add your application URL to the server-config `cors` section:
  - Create or update `config/resources/server_configs/cors.yaml`:
    ```yaml
    name: cors
    value:
      allowedOrigins:
        - "https://localhost:3000"
    ```
  - Or update it at runtime with `PUT /server-config/cors`:
    ```json
    { "allowedOrigins": ["https://localhost:3000"] }
    ```

**Issue**: SSL certificate errors
- Ensure `server.key` and `server.cert` exist in the project root
- Run `./build.sh run_backend` from the project root to auto-generate certificates

## Building for Production

```bash
npm run build
```

The built files will be in the `dist` directory.

## License

Licensed under the Apache License, Version 2.0. You may not use this file except in compliance with the License.

---------------------------------------------------------------------------
(c) Copyright 2026 WSO2 LLC.
