/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package reactsdk provides MCP tools for integrating with the ThunderID React SDK.
package reactsdk

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterTools registers all React SDK tools with the MCP server.
func RegisterTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_integrate_react_sdk",
		Description: "Provides instructions and code snippets for integrating ThunderID authentication via the " +
			"ThunderID React SDK into a React application. Supports two modes: Mode 1 (default) - ThunderID-hosted " +
			"login pages with redirect-based OAuth 2.0/OIDC flow. Mode 2 - Self-hosted login pages using Flow API " +
			"or direct API calls for custom authentication UI.",
		Annotations: &mcp.ToolAnnotations{
			Title:          "Integrate React SDK",
			IdempotentHint: true,
		},
	}, integrateReactSDK)
}

// integrateReactSDK handles the integrate_react_sdk tool call.
func integrateReactSDK(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input integrateReactSDKInput,
) (*mcp.CallToolResult, integrateReactSDKOutput, error) {
	rawInstructions := `
# ThunderID Authentication – React Integration Instructions

## Two Integration Modes

ThunderID supports two ways to integrate React authentication:

- **Mode 1 (Default - Recommended)**: ThunderID-hosted login pages with redirect-based
  OAuth 2.0/OIDC flow using the ThunderID React SDK. Minimal configuration,
  recommended for most use cases.
- **Mode 2**: Self-hosted custom login pages using direct API calls or flow
  orchestration for maximum control. Refer to the React Vanilla Sample or React
  API-Based Sample for implementation details.

This guide covers both integration modes:

- **Mode 1** (default): ThunderID-hosted login with redirect-based OAuth 2.0/OIDC.
- **Mode 2**: Self-hosted app-native login using alternate API-driven patterns.

Mode 1 remains the recommended default path and uses the **ThunderID React SDK**
for a minimal, declarative authentication flow.

## Project Context

This project is a **React application** that integrates **ThunderID
authentication** using the **ThunderID React SDK**, covering both
ThunderID-hosted login (Mode 1) and app-native/self-hosted flows (Mode 2).

The goal is to demonstrate a **minimal, declarative authentication flow** with minimal setup.
This project intentionally avoids custom logic, hooks, and advanced configuration.

---

## SDK & Platform

- **SDK**: @thunderid/react
- **Authentication Mode**: ThunderID-hosted login (redirect-based)
- **Base URL**: https://localhost:8090 (or ThunderID instance URL)
- **Client ID**: <CREATED_APP'S_CLIENT_ID>

---

## CRITICAL: Mode 1 Provider Configuration Rules (MUST FOLLOW EXACTLY)

For **Mode 1 only**, ~ThunderIDProvider~ **MUST** be configured using the
following props.

Always create a NEW public SPA application with token endpoint method as none and use the
client id. Use the application's URL as the redirect URL.

Use the thunderid_get_application_templates tool to get the template and defaults(e.g. user attributes and scopes)
before creating the application.

No variations, no abstractions, no helper objects.

**Mode 2 note:** Method 2 uses an alternate ~ThunderIDProvider~ shape with
~applicationId~ (instead of ~clientId~), as shown in the Mode 2 examples.

### ✅ REQUIRED Provider Configuration

~~~jsx
import { ThunderIDProvider } from '@thunderid/react'

<ThunderIDProvider
  clientId="<client-id>"
  baseUrl="https://localhost:8090"
  afterSignOutUrl="<APP_HOMEPAGE_URL>"
>
  <App />
</ThunderIDProvider>
~~~

**~afterSignOutUrl~ — set this to your app's homepage (e.g. ~http://localhost:5173~).**
Without it, the user may be redirected to an unexpected page after signing out.
Use ~window.location.origin~ or a hardcoded homepage URL. This should match the
homepage of the deployed application in production.

### 🚨 FORBIDDEN Patterns

**For Mode 1, NEVER** do any of the following:

- ❌ ~const config = { ... }; <ThunderIDProvider {...config} />~
- ❌ Extract props to variables
- ❌ Add props other than ~clientId~, ~baseUrl~, and ~afterSignOutUrl~
- ❌ Use different prop names or aliases

---

## Application Structure

### Entry Point (main.jsx or index.jsx)

~~~jsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { ThunderIDProvider } from '@thunderid/react'
import App from './App.jsx'
import './index.css'

createRoot(document.getElementById('root')).render(
  <StrictMode>
    <ThunderIDProvider
      clientId="<client-id>"
      baseUrl="https://localhost:8090"
      afterSignOutUrl={window.location.origin}
    >
      <App />
    </ThunderIDProvider>
  </StrictMode>
)
~~~

---

## Authentication Components

### Using Pre-built Components (Recommended for Simplicity)

The SDK provides declarative components for handling auth states:

#### 1. Sign In/Out Buttons

~~~jsx
import { SignInButton, SignOutButton } from '@thunderid/react'

function Navigation() {
  return (
    <nav>
      <SignInButton>Sign In</SignInButton>
      <SignOutButton>Sign Out</SignOutButton>
    </nav>
  )
}
~~~

#### 2. Sign Up (Self-Registration)

No custom React component is needed for signup. When self-registration is enabled, the ThunderID-hosted
sign-in page automatically renders a **Sign Up** link.

**Two conditions must both be met:**

**1. The application must have registration enabled.**

Use ~thunderid_update_application~ to set ~isRegistrationFlowEnabled~ and point it at a registration flow.
Also add the user type name to ~allowedUserTypes~:

~~~json
{
  "inboundAuthConfig": [{
    "isRegistrationFlowEnabled": true,
    "registrationFlowId": "<registration-flow-id>",
    "allowedUserTypes": ["<user-type-name>"]
  }]
}
~~~

**2. The user type must have ~allowSelfRegistration~ enabled.**

Use ~thunderid_list_user_types~ to find user types. The user type(s) listed in ~allowedUserTypes~ above
must have ~allowSelfRegistration: true~. Without this, the sign-in page will not show the Sign Up link
even if the application has registration enabled.

Once both conditions are met, the hosted sign-in page shows the Sign Up link automatically —
no changes to your React app are required.

#### 3. Conditional Rendering Based on Auth State

~~~jsx
import { SignedIn, SignedOut, Loading, SignInButton, SignOutButton } from '@thunderid/react'

function App() {
  return (
    <>
      <Loading>
        <div>Loading authentication...</div>
      </Loading>

      <SignedOut>
        <h1>Welcome! Please sign in.</h1>
        <SignInButton>Sign In</SignInButton>
      </SignedOut>

      <SignedIn>
        <h1>Welcome back!</h1>
        <SignOutButton>Sign Out</SignOutButton>
      </SignedIn>
    </>
  )
}
~~~

#### 4. Display User Information

**Prerequisites — configure the application before using user attributes:**

User attributes are only available in the React SDK if the application's inbound OAuth config explicitly
includes them. Use the ~thunderid_update_application~ tool to set this on the application:

~~~json
{
  "inboundAuthConfig": [{
    "type": "oauth2",
    "config": {
      "token": {
        "idToken": {
          "userAttributes": ["email", "name", "given_name", "family_name", "picture"]
        }
      },
      "userInfo": {
        "responseType": "JSON",
        "userAttributes": ["email", "name", "given_name", "family_name", "picture"]
      }
    }
  }]
}
~~~

The OAuth scopes requested at login must also cover the attributes you need (e.g. ~openid profile email~).
Attributes not listed in ~userAttributes~ above will not appear in the ID token or the ~/oauth2/userinfo~
response regardless of requested scopes.

**How user data reaches the SDK:**
- The ~User~ component (and ~useThunderID().user~) reads attributes from the **ID token** returned at login.
- For the freshest data (e.g. after a profile update), call ~/oauth2/userinfo~ directly — it is always authoritative.

**PREFERRED:** Use the ~User~ component from ~@thunderid/react~ with render props pattern:

~~~jsx
import { SignedIn, User } from '@thunderid/react'

function UserProfile() {
  return (
    <SignedIn>
      <div>
        <h2>User Profile</h2>
        <User>
          {(user) => user && (
            <>
              {user.picture && (
                <img
                  src={user.picture}
                  alt={user.name || 'User avatar'}
                  style={{ width: '80px', height: '80px', borderRadius: '50%' }}
                />
              )}
              <p>Name: {user?.name}</p>
              <p>Email: {user.email}</p>
              <p>First Name: {user.given_name}</p>
              <p>Last Name: {user.family_name}</p>
            </>
          )}
        </User>
      </div>
    </SignedIn>
  )
}
~~~

---

## Using the Hook (Advanced/Programmatic Control Only)

The ~useThunderID~ hook should only be used when you need programmatic control:

~~~jsx
import { useThunderID } from '@thunderid/react'

function CustomComponent() {
  const { isSignedIn, user, signIn, signOut, loading, error } = useThunderID()

  if (loading) {
    return <div>Loading...</div>
  }

  if (error) {
    return <div>Error: {error.message}</div>
  }

  return (
    <div>
      {isSignedIn ? (
        <>
          <p>Welcome, {user?.displayName}!</p>
          <button onClick={signOut}>Sign Out</button>
        </>
      ) : (
        <button onClick={signIn}>Sign In</button>
      )}
    </div>
  )
}
~~~

**Important:** The ~useThunderID~ hook must be used within a component that is a descendant of ~ThunderIDProvider~.

---

## Route Protection

### Option 1: Using SDK Control Components

~~~jsx
import { SignedIn, SignedOut } from '@thunderid/react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/signin" element={<SignInPage />} />
        <Route
          path="/dashboard"
          element={
            <SignedIn fallback={<Navigate to="/signin" />}>
              <Dashboard />
            </SignedIn>
          }
        />
      </Routes>
    </BrowserRouter>
  )
}
~~~

### Option 2: Using React Router Integration

~~~bash
npm install @thunderid/react-router
~~~

~~~jsx
import { ProtectedRoute } from '@thunderid/react-router'

<Route
  path="/dashboard"
  element={
    <ProtectedRoute redirectTo="/signin">
      <Dashboard />
    </ProtectedRoute>
  }
/>
~~~

### Option 3: Custom Implementation

~~~jsx
import { useThunderID } from '@thunderid/react'
import { Navigate } from 'react-router-dom'

function ProtectedRoute({ children }) {
  const { isSignedIn, loading } = useThunderID()

  if (loading) {
    return <div>Loading...</div>
  }

  if (!isSignedIn) {
    return <Navigate to="/signin" replace />
  }

  return children
}
~~~

---

## Accessing Protected APIs

### Using SDK Built-in HTTP Client (webWorker storage)

~~~jsx
import { useThunderID } from '@thunderid/react'
import { useEffect, useState } from 'react'

function UserData() {
  const { http, isSignedIn } = useThunderID()
  const [data, setData] = useState(null)

  useEffect(() => {
    if (!isSignedIn) return

    (async () => {
      try {
        const response = await http.request({
          url: 'https://localhost:8090/scim2/Me',
          method: 'GET',
          headers: {
            'Accept': 'application/json',
            'Content-Type': 'application/scim+json'
          }
        })
        setData(response.data)
      } catch (error) {
        console.error('API Error:', error)
      }
    })()
  }, [http, isSignedIn])

  return <div>{data && <pre>{JSON.stringify(data, null, 2)}</pre>}</div>
}
~~~

**Note:** The ~http~ module automatically attaches the access token to requests.

### Using Custom HTTP Client (sessionStorage/localStorage)

~~~jsx
import { useThunderID } from '@thunderid/react'

async function fetchUserData() {
  const { getAccessToken, isSignedIn } = useThunderID()

  if (!isSignedIn) return

  const token = await getAccessToken()

  const response = await fetch('https://localhost:8090/scim2/Me', {
    headers: {
      'Authorization': ~Bearer ${token}~,
      'Accept': 'application/json'
    }
  })

  return response.json()
}
~~~

---

## Complete Example

~~~jsx
// main.jsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { ThunderIDProvider } from '@thunderid/react'
import App from './App.jsx'
import './index.css'

createRoot(document.getElementById('root')).render(
  <StrictMode>
    <ThunderIDProvider
      clientId="<client-id>"
      baseUrl="https://localhost:8090"
      afterSignOutUrl={window.location.origin}
    >
      <App />
    </ThunderIDProvider>
  </StrictMode>
)
~~~

~~~jsx
// App.jsx
import { SignedIn, SignedOut, SignInButton, SignOutButton, Loading, User } from '@thunderid/react'

function App() {
  return (
    <div className="app">
      <header>
        <h1>ThunderID Auth Demo</h1>
        <Loading>
          <div>Loading...</div>
        </Loading>
      </header>

      <main>
        <SignedOut>
          <div className="welcome">
            <h2>Welcome!</h2>
            <p>Please sign in to continue</p>
            <SignInButton>Sign In</SignInButton>
          </div>
        </SignedOut>

        <SignedIn>
          <div className="dashboard">
            <User>
              {(user) => (
                <>
                  <h2>Welcome, {user?.displayName}!</h2>
                  <div className="user-info">
                    <p><strong>Email:</strong> {user?.email}</p>
                    <p><strong>Username:</strong> {user?.username}</p>
                  </div>
                </>
              )}
            </User>
            <SignOutButton>Sign Out</SignOutButton>
          </div>
        </SignedIn>
      </main>
    </div>
  )
}

export default App
~~~

---

## Method 2: ThunderID App Native Authentication with React (Vite)

This guide shows how to integrate ThunderID App Native authentication into a React
app using ~@thunderid/react~, based on this sample project.

### Prerequisites

- A ThunderID application already created.
- Your ThunderID **Application ID (UUID)**.
- Node.js and npm installed.

### 1) Create a new Vite project (or use existing)

If starting fresh, create a new Vite React app:

~~~bash
npm create vite@latest my-app -- --template react
cd my-app
npm install
~~~

### 2) Install dependencies

~~~bash
npm install @thunderid/react
~~~

If the dependency already exists in ~package.json~, you can skip the above steps.

### 3) Wrap your app with ~ThunderIDProvider~

Update ~src/main.jsx~ to configure the authentication provider.

~~~jsx
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import App from './App.jsx';
import { ThunderIDProvider } from '@thunderid/react';
import './index.css';

createRoot(document.getElementById('root')).render(
  <StrictMode>
    <ThunderIDProvider
      baseUrl="https://localhost:8090"
      applicationId="<THUNDERID_APP_ID>"
    >
      <App />
    </ThunderIDProvider>
  </StrictMode>
);
~~~

Replace ~<THUNDERID_APP_ID>~ with your real application UUID from ThunderID.

### 4) Add Sign-In UI

You can quickly enable login by using the built-in ~SignIn~ component.

~~~jsx
import { SignIn } from '@thunderid/react';

function App() {
  return (
    <div>
      <h1>My React App</h1>
      <SignIn />
    </div>
  );
}

export default App;
~~~

This component starts the App Native authentication flow when the user signs in.

### 5) Run the app

~~~bash
npm run dev
~~~

Open the app in your browser and click the sign-in action.

### Optional: Move auth config to environment variables

For cleaner configuration, store values in ~.env~:

~~~bash
VITE_THUNDERID_BASE_URL=https://localhost:8090
VITE_THUNDERID_APP_ID=<THUNDERID_APP_ID>
~~~

Then use them in ~src/main.jsx~:

~~~jsx
<ThunderIDProvider
  baseUrl={import.meta.env.VITE_THUNDERID_BASE_URL}
  applicationId={import.meta.env.VITE_THUNDERID_APP_ID}
>
  <App />
</ThunderIDProvider>
~~~

### Do's and Don'ts

#### ✅ Do

- Do replace ~<THUNDERID_APP_ID>~ with the exact UUID from your ThunderID app registration.
- Do keep auth settings (~baseUrl~, app ID) environment-specific (dev/stage/prod).
- Do keep ~ThunderIDProvider~ high in the component tree (usually in ~src/main.jsx~).
- Do validate the ~baseUrl~ and cert setup when running locally over HTTPS.
- Do use the latest compatible ~@thunderid/react~ version for your app.

#### ❌ Don't

- Don't hard-code production credentials across multiple files.
- Don't commit secret or environment-specific values into source control.
- Don't initialize multiple auth providers in parallel for the same React tree.
- Don't assume localhost settings will work unchanged in production.
- Don't bypass proper sign-in flow with manual token handling unless absolutely necessary.

### Quick troubleshooting

- **Sign-in not starting:** Verify ~applicationId~ and ~baseUrl~.
- **Invalid app/client errors:** Re-check the ThunderID app registration and copied UUID.
- **Local HTTPS issues:** Confirm your local endpoint and certificate trust setup.

---

For complete implementation examples of Method 2 authentication, refer to:

- **React Vanilla Sample** - Demonstrates both:
  - App-native authentication using Flow Orchestration API
  - Standard OAuth 2.0 / OIDC with custom UI

Both samples show how to build custom authentication UIs while leveraging ThunderID's authentication capabilities.

---

## Migrating from Mode 1 to Mode 2

Use this guide when switching an existing Mode 1 (redirect-based) integration to Mode 2 (app-native).

### What changes

| Area | Mode 1 | Mode 2 |
|---|---|---|
| Provider prop | ~clientId~ | ~applicationId~ |
| Sign-in UI | ~<SignInButton />~ (redirect) | ~<SignIn />~ (inline app-native flow) |
| Sign-out | ~<SignOutButton />~ | ~<SignOutButton />~ / ~signOut()~ |
| App registration | Public SPA (token endpoint: none) | Standard application — use the Application ID (UUID) |

### Step 1: Get your Application ID

In Mode 2 the provider is configured with the application's UUID (~applicationId~), not a client ID string.

Use ~thunderid_list_applications~ to find your application and copy its UUID. If you need a new application,
use ~thunderid_create_application~ and note the returned UUID.

### Step 2: Update the provider

Replace ~clientId~ with ~applicationId~ in ~ThunderIDProvider~:

~~~jsx
// Before (Mode 1)
<ThunderIDProvider
  clientId="<client-id>"
  baseUrl="https://localhost:8090"
>

// After (Mode 2)
<ThunderIDProvider
  applicationId="<application-uuid>"
  baseUrl="https://localhost:8090"
>
~~~

### Step 3: Update runtime/environment config

If your app reads config from a file (e.g. ~runtime.json~) or environment variables, rename the key:

~~~json
// Before
{ "clientId": "REACT_SDK_SAMPLE", "baseUrl": "https://localhost:8090" }

// After
{ "applicationId": "<application-uuid>", "baseUrl": "https://localhost:8090" }
~~~

Update the corresponding config loader (~config.ts~ or equivalent) to read ~applicationId~ instead of ~clientId~.

### Step 4: Replace the sign-in component

Replace ~<SignInButton />~ with the ~<SignIn />~ component. ~<SignIn />~ renders the app-native
authentication UI inline — no browser redirect occurs and no separate sign-in page or route is needed.

The ~<SignIn />~ component handles the entire auth flow (username/password fields, step-up prompts, etc.)
within your app itself.

~~~jsx
// Before (Mode 1)
import { SignedIn, SignedOut, SignInButton } from '@thunderid/react'

function App() {
  return (
    <>
      <SignedIn><Dashboard /></SignedIn>
      <SignedOut><SignInButton /></SignedOut>
    </>
  )
}

// After (Mode 2)
import { SignedIn, SignedOut, SignIn } from '@thunderid/react'

function App() {
  return (
    <>
      <SignedIn><Dashboard /></SignedIn>
      <SignedOut><SignIn /></SignedOut>
    </>
  )
}
~~~

If you previously had a dedicated sign-in route (e.g. ~<Route path="/signin" element={<SignInPage />} />~)
just to host the ~<SignInButton />~, remove that route entirely and render ~<SignIn />~ directly in your
main layout.

### Step 5: Update sign-out

In Mode 2, ~signOut()~ ends the session and the ~<SignIn />~ component re-renders automatically.

~~~jsx
// Mode 1
import { SignOutButton } from '@thunderid/react'
<SignOutButton>Sign Out</SignOutButton>

// Mode 2
import { SignOutButton } from '@thunderid/react'
<SignOutButton>Sign Out</SignOutButton>

// Or with the hook (Mode 2)
const { signOut } = useThunderID()
<button onClick={() => signOut()}>Sign Out</button>
~~~

If you were calling ~signIn()~ immediately after ~signOut()~ to return to the login screen, remove that
call — ~<SignIn />~ handles re-rendering automatically when the user is signed out.

### Step 6: Validation checks

Update any startup validation that checks for ~clientId~ to check for ~applicationId~ instead.

### Quick checklist

- [ ] ~ThunderIDProvider~ uses ~applicationId~ (UUID), not ~clientId~
- [ ] ~runtime.json~ / env vars updated to ~applicationId~
- [ ] Config loader reads ~applicationId~
- [ ] ~<SignInButton />~ replaced with ~<SignIn />~
- [ ] Sign-out uses ~<SignOutButton>~ or ~signOut()~
- [ ] No calls to ~signIn()~ after ~signOut()~

---

## Custom Signup Form

### How it works

The signup flow is server-driven. When the ~<SignUp>~ component mounts, it calls the ~/flow/execute~ API
to initialize a ~REGISTRATION~ flow. The server responds with a list of **components** (fields, buttons,
dividers, etc.) that describe what to render. On each form submission the same API is called again with
the user's inputs until ~flowStatus === 'COMPLETE'~.

### Provider setup for signup

Wrap your app with ~ThunderIDProvider~. The minimum required config is ~baseUrl~ and ~clientId~.

~~~tsx
import { ThunderIDProvider } from '@thunderid/react';

<ThunderIDProvider
  baseUrl="https://localhost:8090"
  clientId="your-client-id"
  afterSignInUrl="http://localhost:3000"
  afterSignOutUrl="http://localhost:3000"
  scopes={['openid', 'profile', 'email']}
>
  <App />
</ThunderIDProvider>
~~~

#### Do you need ~applicationId~?

~applicationId~ is **optional but recommended**.

| Scenario | Behavior without ~applicationId~ |
|---|---|
| Branding (colors, logo) | Falls back to org-level defaults |
| Signup flow initialization | SDK reads ~applicationId~ from URL query param as fallback, otherwise omits it |
| Flow meta / i18n | SDK uses org-level branding |

If you set it in the provider, it propagates automatically to the signup flow:

~~~tsx
<ThunderIDProvider
  baseUrl="https://localhost:8090"
  clientId="your-client-id"
  applicationId="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"  // optional UUID
>
~~~

Alternatively you can pass ~applicationId~ as a URL query param (~?applicationId=...~) and the SDK will
pick it up automatically — useful when the signup page is shared across multiple apps.

### Create the signup page/route

Point users to a dedicated page that renders the ~<SignUp>~ component. Configure ~signUpUrl~ in the
provider to that page:

~~~tsx
<ThunderIDProvider
  ...
  signUpUrl="http://localhost:3000/signup"
>
~~~

### Render the signup form

#### Option A — Pre-built UI (zero effort)

~~~tsx
import { SignUp } from '@thunderid/react';

const SignUpPage = () => (
  <SignUp
    afterSignUpUrl="/welcome"
    onComplete={(response) => console.log('Done', response)}
    onError={(err) => console.error(err)}
    size="medium"       // 'small' | 'medium' | 'large'
    variant="outlined"  // 'elevated' | 'outlined' | 'flat'
  />
);
~~~

The component renders whatever fields the server sends (username, email, password, etc.) and handles all
flow steps automatically.

#### Option B — Custom UI with render props

Pass a ~children~ function. The SDK still owns the flow state; you own the markup.

~~~tsx
import { SignUp } from '@thunderid/react';

const SignUpPage = () => (
  <SignUp
    afterSignUpUrl="/welcome"
    onError={(err) => console.error(err)}
    onComplete={(response) => console.log('Signup complete', response)}
  >
    {({
      components,        // server-driven component list for the current step
      values,            // form field values — keyed by component.ref
      fieldErrors,       // per-field validation errors
      touched,           // whether a field has been blurred
      isLoading,         // true while the flow API call is in-flight
      messages,          // server error / info messages [{message, type}]
      handleInputChange, // (fieldName, value) => void
      handleSubmit,      // (component, values) => Promise<void> — pass the button component
    }) => (
      <div>
        <h1>Create an Account</h1>

        {/* Server messages (errors, warnings) */}
        {messages.map((msg, i) => (
          <p key={i} style={{ color: msg.type === 'error' ? 'red' : 'blue' }}>
            {msg.message}
          </p>
        ))}

        {/* Render text / email / password inputs */}
        {components
          .filter(c => ['TEXT_INPUT', 'EMAIL_INPUT', 'PASSWORD_INPUT'].includes(c.type))
          .map(component => (
            <div key={component.ref || component.id}>
              <label>{component.label}</label>
              <input
                type={component.type === 'PASSWORD_INPUT' ? 'password' : 'text'}
                value={values[component.ref] ?? ''}
                onChange={e => handleInputChange(component.ref, e.target.value)}
              />
              {touched[component.ref] && fieldErrors[component.ref] && (
                <span style={{ color: 'red' }}>{fieldErrors[component.ref]}</span>
              )}
            </div>
          ))}

        {/* Submit button */}
        {components
          .filter(c => c.type === 'BUTTON')
          .map(component => (
            <button
              key={component.id}
              onClick={() => handleSubmit(component, values)}
              disabled={isLoading}
            >
              {isLoading ? 'Creating account...' : component.label ?? 'Sign Up'}
            </button>
          ))}
      </div>
    )}
  </SignUp>
);
~~~

### Handle completion

~~~tsx
<SignUp
  afterSignUpUrl="/welcome"           // auto-redirect on success (default behavior)
  shouldRedirectAfterSignUp={true}    // default: true
  onComplete={(response) => {
    // Called when flowStatus === 'COMPLETE'
    // response.redirectUrl is set when an OAuth assertion was completed
    console.log('Signup complete', response);
  }}
  onError={(err) => {
    // Called on API errors or unrecoverable flow errors
    console.error(err);
  }}
/>
~~~

If you need custom redirect logic (e.g. Next.js ~router.push~), set ~shouldRedirectAfterSignUp={false}~
and handle it in ~onComplete~.

### Render props reference

| Prop | Type | Description |
|---|---|---|
| ~components~ | ~any[]~ | Server-driven component tree for the current flow step |
| ~values~ | ~Record<string, string>~ | Form values keyed by ~component.ref~ |
| ~fieldErrors~ | ~Record<string, string>~ | Per-field validation error messages |
| ~touched~ | ~Record<string, boolean>~ | Whether the user has interacted with a field |
| ~isLoading~ | ~boolean~ | Flow API call is in-flight |
| ~isValid~ | ~boolean~ | All required fields pass validation |
| ~messages~ | ~{message: string, type: string}[]~ | Server messages |
| ~error~ | ~Error \| null~ | API-level error |
| ~title~ | ~string~ | Flow step title (from server or i18n) |
| ~subtitle~ | ~string~ | Flow step subtitle |
| ~handleInputChange~ | ~(name, value) => void~ | Update a field value |
| ~handleSubmit~ | ~(component, data?) => Promise<void>~ | Advance the flow; pass button and current ~values~ |
| ~validateForm~ | ~() => {fieldErrors, isValid}~ | Manually trigger validation |

### Key rules

- **Use ~component.ref~ as the field key**, not ~component.id~. The SDK maps input names to ~ref~
  when building the ~inputs~ payload sent to the server.
- **Social / OAuth signup** (Google, GitHub, etc.) is handled automatically via a popup —
  your render-prop UI does not need to handle ~type: 'REDIRECTION'~ responses.
- **Passkey registration** is also handled automatically when
  ~additionalData.passkeyCreationOptions~ is present in the flow response.
- The flow can have **multiple steps** (e.g. email → OTP → password). The ~components~ array
  updates after each successful submission — your UI re-renders the new step automatically.

### Common ~component.type~ values

| Type | Renders as |
|---|---|
| ~TEXT_INPUT~ | Plain text field |
| ~EMAIL_INPUT~ | Email field |
| ~PASSWORD_INPUT~ | Password field |
| ~SELECT~ | Dropdown |
| ~BUTTON~ | Submit / action button |
| ~DIVIDER~ | Visual separator |
| ~HEADING~ | Title/subtitle text |

---

## Best Practices

### ✅ DO:

- Use declarative components (~<SignedIn>~, ~<SignedOut>~, ~<Loading>~) for UI state
- Use pre-built action components (~<SignInButton>~, ~<SignOutButton>~)
- Keep the provider configuration minimal and explicit
- Use the ~useThunderID~ hook only when programmatic control is needed
- Handle loading and error states properly

### ❌ DON'T:

- Don't create custom authentication logic unless absolutely necessary
- Don't manipulate tokens manually
- Don't store tokens in localStorage unless using the SDK's storage mechanism
- Don't add unnecessary configuration to the provider
- Don't use the hook outside of components wrapped by ~ThunderIDProvider~

---

## Common Patterns

### Pattern 1: Simple Auth-Gated App

~~~jsx
function App() {
  return (
    <>
      <SignedOut>
        <LandingPage />
      </SignedOut>
      <SignedIn>
        <Dashboard />
      </SignedIn>
    </>
  )
}
~~~

### Pattern 2: Navigation Bar with Conditional Auth

~~~jsx
function NavBar() {
  return (
    <nav>
      <Logo />
      <SignedOut>
        <SignInButton>Login</SignInButton>
      </SignedOut>
      <SignedIn>
        <UserMenu />
        <SignOutButton>Sign Out</SignOutButton>
      </SignedIn>
    </nav>
  )
}
~~~

### Pattern 3: Loading State Handling

~~~jsx
function App() {
  return (
    <>
      <Loading fallback={null}>
        <div className="spinner">Authenticating...</div>
      </Loading>

      <SignedIn>
        <Dashboard />
      </SignedIn>
    </>
  )
}
~~~

---

## Troubleshooting

### Issue: Hook Error "useThunderID must be used within ThunderIDProvider"

**Solution:** Ensure the component using ~useThunderID~ is a child of ~<ThunderIDProvider>~

### Issue: Infinite redirect loop

**Solution:** Check that ~baseUrl~ and ~clientId~ are correct. Verify token validation settings.

### Issue: User object is null after sign in

**Solution:** Ensure authentication has completed. Check for any errors in the console.

### Issue: CORS errors

**Solution:** Configure CORS settings in ThunderID to allow your app's origin.

---

## References

- [ThunderID React SDK Docs](/docs/next/sdks/react/overview)
- [ThunderIDProvider Configuration](/docs/next/sdks/react/apis/contexts/thunderid-provider)
- [SDK Components](/docs/next/sdks/react/apis/components/sign-in-button)
- [useThunderID Hook](/docs/next/sdks/react/apis/hooks/use-thunderid)
- [Protecting Routes](/docs/next/sdks/react/guides/protecting-routes/overview)
- [Accessing Protected APIs](/docs/next/sdks/react/guides/accessing-protected-apis)
`
	instructions := strings.ReplaceAll(rawInstructions, "~", "`")

	snippets := `
import { ThunderIDProvider } from '@thunderid/react';

// Main Provider Setup
<ThunderIDProvider
  clientId="<client-id>"
  baseUrl="https://localhost:8090"
  afterSignOutUrl={window.location.origin}
>
  <App />
</ThunderIDProvider>
`

	// Template the URL if provided
	if input.ServerURL != "" {
		instructions = strings.ReplaceAll(instructions, "https://localhost:8090", input.ServerURL)
		snippets = strings.ReplaceAll(snippets, "https://localhost:8090", input.ServerURL)
	}

	return nil, integrateReactSDKOutput{
		Instructions: instructions,
		CodeSnippets: snippets,
	}, nil
}
