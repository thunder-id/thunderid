# Sample App MFA Authentication Tests

This directory contains end-to-end tests for Multi-Factor Authentication (MFA) login flows using SMS OTP as the second factor.

## Overview

These tests verify the complete MFA authentication process:

1. **First Factor**: Username and password authentication
2. **Second Factor**: SMS OTP verification

The tests use a mock SMS server to capture OTP messages sent by the server, eliminating the need for real SMS providers during testing.

## Test Files

- **`sample-app-login.spec.ts`**: Basic login/logout tests (single-factor authentication)
- **`sample-app-mfa-login.spec.ts`**: MFA tests with SMS OTP (two-factor authentication)

## Quick Start (Automated Setup)

The tests now support **automated setup** of all MFA prerequisites! Simply configure environment variables and run:

```bash
# 1. Configure environment variables in tests/e2e/.env
SAMPLE_APP_URL=https://localhost:3000
SAMPLE_APP_ID=<your-application-id>
SERVER_URL=https://localhost:8090
ADMIN_USERNAME=admin
ADMIN_PASSWORD=admin
SAMPLE_APP_USERNAME=e2e-test-user
SAMPLE_APP_PASSWORD=e2e-test-password
AUTO_SETUP_MFA=true  # Enable automated setup (default)

# 2. Run tests - setup happens automatically!
npm test tests/sample-app-authentication/sample-app-mfa-login.spec.ts
```

The automated setup will:
- ✅ Authenticate as admin
- ✅ Create notification sender pointing to mock SMS server
- ✅ Create MFA authentication flow with SMS OTP
- ✅ Create test user with mobile number
- ✅ Update application to use MFA flow
- ✅ Clean up all resources after tests complete

## Prerequisites

### Minimal Requirements (with Automated Setup)

1. ✅ **Server running** at `https://localhost:8090`
2. ✅ **Sample app running** (e.g., at `https://localhost:3000`)
3. ✅ **Application ID** - Get from server setup

### Full Requirements (Manual Setup)

If you prefer manual setup or `AUTO_SETUP_MFA=false`:

1. ✅ **Server running** at `https://localhost:8090`
2. ✅ **Sample app running** (e.g., at `https://localhost:3000`)
3. ✅ **Admin access token** available for configuration
4. ✅ **Notification sender configured** to use mock SMS server
5. ✅ **MFA authentication flow configured** in server
6. ✅ **Test user with mobile number** created in server
7. ✅ **Application created with MFA flow attached** in server

## Setup Instructions

### Option 1: Automated Setup (Recommended)

1. **Configure Environment Variables**

Copy the example environment file and update with your values:

```bash
cd tests/e2e
cp tests/sample-app-authentication/.env.example .env
# Edit .env with your actual values
```

Or add directly to your `tests/e2e/.env` file:

```env
# Required for automated setup
SAMPLE_APP_URL=https://localhost:3000
SAMPLE_APP_ID=<your-application-id>
SERVER_URL=https://localhost:8090
ADMIN_USERNAME=admin
ADMIN_PASSWORD=admin
SAMPLE_APP_USERNAME=e2e-test-user
SAMPLE_APP_PASSWORD=e2e-test-password

# Optional
MOCK_SMS_SERVER_PORT=8098
AUTO_SETUP_MFA=true  # Set to false to disable automated setup
```

**Getting your Application ID:**
```bash
# Authenticate as admin first, then:
curl -k 'https://localhost:8090/applications' \
  -H "Authorization: Bearer <admin-token>" | jq '.applications[] | {id, name}'
```

2. **Run Tests**

```bash
npm test tests/sample-app-authentication/sample-app-mfa-login.spec.ts
```

That's it! The setup will be performed automatically before tests run.

### Option 2: Manual Setup (Server Configuration)

If you prefer manual configuration or need to troubleshoot, follow these steps:

#### Admin Access Token

Run the following command, replacing `<application_id>` with your sample app ID (created during server setup).

```bash
FLOW_RESPONSE=$(curl -k -s -X POST 'https://localhost:8090/flow/execute' \
  -d '{"applicationId":"<application_id>","flowType":"AUTHENTICATION"}')

EXECUTION_ID=$(echo $FLOW_RESPONSE | jq -r '.executionId')
```

Run the following command with the extracted `executionId`.

```bash
ADMIN_TOKEN_RESPONSE=$(curl -k -s -X POST 'https://localhost:8090/flow/execute' \
  -d '{"executionId":"'$EXECUTION_ID'","inputs":{"username":"admin","password":"admin","requested_permissions":"system"},"action":"action_001"}')

ADMIN_TOKEN=$(echo $ADMIN_TOKEN_RESPONSE | jq -r '.assertion')
```

### Create the Notification Sender

```bash
NOTIFICATION_SENDER_RESPONSE=$(curl -kL -X POST 'https://localhost:8090/notification-senders/message' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "name": "E2E Mock SMS Sender",
    "description": "Mock SMS sender for e2e MFA testing",
    "provider": "custom",
    "properties": [ 
      {                                                      
        "name": "url",                                               
        "value": "http://localhost:8098/send-sms",
        "isSecret": false              
      },                                                                               
      {                      
        "name": "http_method",
        "value": "POST",
        "isSecret": false                 
      },                                             
      {                                         
        "name": "content_type",      
        "value": "JSON",      
        "isSecret": false                      
      }                       
    ]             
  }'                                               
)

NOTIFICATION_SENDER_ID=$(echo $NOTIFICATION_SENDER_RESPONSE | jq -r '.id')
```

### Create the MFA Authentication Flow

```bash
FLOW_RESPONSE=$(curl --location 'https://localhost:8090/flows' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer $ADMIN_TOKEN' \
--data '{
    "handle": "e2e-mfa-auth-flow",
    "name": "E2E MFA Authentication Flow",
    "flowType": "AUTHENTICATION",
    "activeVersion": 3,
    "nodes": [
        {
            "id": "start",
            "type": "START",
            "layout": {
                "size": {
                    "width": 101,
                    "height": 34
                },
                "position": {
                    "x": 62,
                    "y": 87
                }
            },
            "onSuccess": "prompt_credentials"
        },
        {
            "id": "prompt_credentials",
            "type": "PROMPT",
            "layout": {
                "size": {
                    "width": 350,
                    "height": 560
                },
                "position": {
                    "x": 562,
                    "y": 62
                }
            },
            "meta": {
                "components": [
                    {
                        "category": "DISPLAY",
                        "id": "text_001",
                        "label": "{{ t(signin:heading) }}",
                        "resourceType": "ELEMENT",
                        "type": "TEXT",
                        "variant": "HEADING_1"
                    },
                    {
                        "category": "BLOCK",
                        "components": [
                            {
                                "category": "FIELD",
                                "hint": "",
                                "id": "input_001",
                                "inputType": "text",
                                "label": "{{ t(elements:fields.username.label) }}",
                                "placeholder": "{{ t(elements:fields.username.placeholder) }}",
                                "ref": "username",
                                "required": true,
                                "resourceType": "ELEMENT",
                                "type": "TEXT_INPUT"
                            },
                            {
                                "category": "FIELD",
                                "hint": "",
                                "id": "input_002",
                                "inputType": "text",
                                "label": "{{ t(elements:fields.password.label) }}",
                                "placeholder": "{{ t(elements:fields.password.placeholder) }}",
                                "ref": "password",
                                "required": true,
                                "resourceType": "ELEMENT",
                                "type": "PASSWORD_INPUT"
                            },
                            {
                                "category": "ACTION",
                                "eventType": "SUBMIT",
                                "id": "action_001",
                                "label": "{{ t(elements:buttons.submit.text) }}",
                                "resourceType": "ELEMENT",
                                "type": "ACTION",
                                "variant": "PRIMARY"
                            }
                        ],
                        "id": "block_001",
                        "resourceType": "ELEMENT",
                        "type": "BLOCK"
                    }
                ]
            },
            "inputs": [
                {
                    "ref": "input_001",
                    "type": "TEXT_INPUT",
                    "identifier": "username",
                    "required": true
                },
                {
                    "ref": "input_002",
                    "type": "PASSWORD_INPUT",
                    "identifier": "password",
                    "required": true
                }
            ],
            "actions": [
                {
                    "ref": "action_001",
                    "nextNode": "credentials_auth"
                }
            ]
        },
        {
            "id": "credentials_auth",
            "type": "TASK_EXECUTION",
            "layout": {
                "size": {
                    "width": 217,
                    "height": 113
                },
                "position": {
                    "x": 1062,
                    "y": 62
                }
            },
            "inputs": [
                {
                    "ref": "input_001",
                    "type": "TEXT_INPUT",
                    "identifier": "username",
                    "required": true
                },
                {
                    "ref": "input_002",
                    "type": "PASSWORD_INPUT",
                    "identifier": "password",
                    "required": true
                }
            ],
            "executor": {
                "name": "CredentialsAuthExecutor"
            },
            "onSuccess": "authorization_check"
        },
        {
            "id": "authorization_check",
            "type": "TASK_EXECUTION",
            "layout": {
                "size": {
                    "width": 200,
                    "height": 113
                },
                "position": {
                    "x": 1562,
                    "y": 62
                }
            },
            "executor": {
                "name": "AuthorizationExecutor"
            },
            "onSuccess": "send_otp"
        },
        {
            "id": "send_otp",
            "type": "TASK_EXECUTION",
            "layout": {
                "size": {
                    "width": 200,
                    "height": 113
                },
                "position": {
                    "x": 2062,
                    "y": 62
                }
            },
            "inputs": [
                {
                    "ref": "otp_input_24ux",
                    "type": "OTP_INPUT",
                    "identifier": "otp",
                    "required": false
                }
            ],
            "properties": {
                "senderId": "$NOTIFICATION_SENDER_ID"
            },
            "executor": {
                "name": "SMSOTPAuthExecutor",
                "mode": "send"
            },
            "onSuccess": "view_s2t2"
        },
        {
            "id": "verify_otp",
            "type": "TASK_EXECUTION",
            "layout": {
                "size": {
                    "width": 200,
                    "height": 113
                },
                "position": {
                    "x": 3062,
                    "y": 62
                }
            },
            "inputs": [
                {
                    "ref": "otp_input_24ux",
                    "type": "OTP_INPUT",
                    "identifier": "otp",
                    "required": false
                }
            ],
            "properties": {
                "senderId": "$NOTIFICATION_SENDER_ID"
            },
            "executor": {
                "name": "SMSOTPAuthExecutor",
                "mode": "verify"
            },
            "onSuccess": "auth_assert"
        },
        {
            "id": "auth_assert",
            "type": "TASK_EXECUTION",
            "layout": {
                "size": {
                    "width": 244,
                    "height": 113
                },
                "position": {
                    "x": 3562,
                    "y": 62
                }
            },
            "executor": {
                "name": "AuthAssertExecutor"
            },
            "onSuccess": "end"
        },
        {
            "id": "end",
            "type": "END",
            "layout": {
                "size": {
                    "width": 85,
                    "height": 34
                },
                "position": {
                    "x": 4062,
                    "y": 87
                }
            }
        },
        {
            "id": "view_s2t2",
            "type": "PROMPT",
            "layout": {
                "size": {
                    "width": 350,
                    "height": 522
                },
                "position": {
                    "x": 2591,
                    "y": 37
                }
            },
            "meta": {
                "components": [
                    {
                        "category": "DISPLAY",
                        "id": "text_nwu6",
                        "label": "Verify OTP",
                        "resourceType": "ELEMENT",
                        "type": "TEXT",
                        "variant": "HEADING_3"
                    },
                    {
                        "category": "BLOCK",
                        "components": [
                            {
                                "category": "FIELD",
                                "hint": "",
                                "id": "otp_input_24ux",
                                "inputType": "text",
                                "label": "Enter the code sent to your mobile",
                                "placeholder": "",
                                "ref": "otp",
                                "required": false,
                                "resourceType": "ELEMENT",
                                "type": "OTP_INPUT"
                            },
                            {
                                "category": "ACTION",
                                "eventType": "TRIGGER",
                                "id": "action_s76e",
                                "label": "Verify",
                                "resourceType": "ELEMENT",
                                "type": "ACTION",
                                "variant": "PRIMARY"
                            },
                            {
                                "category": "ACTION",
                                "eventType": "SUBMIT",
                                "id": "resend_6o42",
                                "label": "Resend OTP",
                                "resourceType": "ELEMENT",
                                "type": "RESEND"
                            }
                        ],
                        "id": "block_gwme",
                        "resourceType": "ELEMENT",
                        "type": "BLOCK"
                    }
                ]
            },
            "inputs": [
                {
                    "ref": "otp_input_24ux",
                    "type": "OTP_INPUT",
                    "identifier": "otp",
                    "required": false
                }
            ],
            "actions": [
                {
                    "ref": "action_s76e",
                    "nextNode": "verify_otp"
                },
                {
                    "ref": "resend_6o42",
                    "nextNode": "send_otp"
                }
            ]
        }
    ],
    "createdAt": "2026-01-07 09:58:36",
    "updatedAt": "2026-01-07 12:26:03"
}')

FLOW_ID=$(echo $FLOW_RESPONSE | jq -r '.id')
```

### Create a User with Mobile No
```bash
USER_RESPONSE=$(curl --location 'https://localhost:8090/users' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer $ADMIN_TOKEN' \
--data-raw '{
  "type": "Person",
  "organizationUnit": "6d7029a6-1092-4f3e-ab54-a14662ac7045",
  "attributes": {
    "username": "e2e-test-user",
    "password": "e2e-test-password",
    "given_name": "E2E User",
    "email": "e2e@example.com",
    "mobileNumber": "+12345678920"
  }
}')
```

### Update the sample application

Follow these steps:
1. Fetch all applications from the server
2. Find the "React SDK Sample" application
3. Get its full configuration
4. Update the `auth_flow_id` with the new MFA flow
5. Send a PUT request to update the application

**Or manually update via curl:**

```bash
# Get the React SDK Sample application ID
APP_RESPONSE=$(curl -k -s -X GET 'https://localhost:8090/applications' \
  -H "Authorization: Bearer $ADMIN_TOKEN")

APP_ID=$(echo $APP_RESPONSE | jq -r '.applications[] | select(.name == "React SDK Sample") | .id')

# Get full application details
APP_DETAILS=$(curl -k -s -X GET "https://localhost:8090/applications/$APP_ID" \
  -H "Authorization: Bearer $ADMIN_TOKEN")

# Update with new flow ID
UPDATED_APP=$(echo "$APP_DETAILS" | jq --arg flow_id "$FLOW_ID" '.auth_flow_id = $flow_id')

# PUT the updated application
curl -k -X PUT "https://localhost:8090/applications/$APP_ID" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "$UPDATED_APP"
```

> **Note:** When using manual setup, ensure all resources are created before running tests. With automated setup (`AUTO_SETUP_MFA=true`), all resources are created and cleaned up automatically.

## Environment Variables Reference

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SAMPLE_APP_URL` | Yes | - | URL of the sample app (e.g., `https://localhost:3000`) |
| `SAMPLE_APP_ID` | Yes* | - | Application ID in the server (*required for automated setup) |
| `SERVER_URL` | No | `https://localhost:8090` | URL of the server |
| `ADMIN_USERNAME` | No | `admin` | Admin username for the server |
| `ADMIN_PASSWORD` | No | `admin` | Admin password for the server |
| `SAMPLE_APP_USERNAME` | No | `e2e-test-user` | Test user username |
| `SAMPLE_APP_PASSWORD` | No | `e2e-test-password` | Test user password |
| `MOCK_SMS_SERVER_PORT` | No | `8098` | Port for mock SMS server |
| `AUTO_SETUP_MFA` | No | `true` | Enable/disable automated setup |

## Running the Tests

### Run with Automated Setup (Default)

```bash
cd tests/e2e

# Run only MFA tests
npm test tests/sample-app-authentication/sample-app-mfa-login.spec.ts

# Run all sample app tests
npm test tests/sample-app-authentication/
```

### Run with Manual Setup

```bash
cd tests/e2e

# Disable automated setup
AUTO_SETUP_MFA=false npm test tests/sample-app-authentication/sample-app-mfa-login.spec.ts
```

### Run with UI Mode (Debugging)

```bash
npm run ui
```

### Run with Debug Mode

```bash
npm run test:debug tests/sample-app-authentication/sample-app-mfa-login.spec.ts
```

## Test Cases

### TC001: Complete MFA Login Flow
**Test**: `TC001: Complete MFA login flow with username/password + SMS OTP`

**Steps:**
1. Navigate to sample app home page
2. Verify home page loaded
3. Click "Sign In" button
4. Verify login page is displayed
5. Enter username and password (first factor)
6. Submit login form
7. Verify OTP verification page is displayed (or skip if MFA not configured)
8. Wait for SMS to be sent (2 second delay)
9. Retrieve OTP from mock SMS server
10. Validate SMS message received and OTP extracted
11. Verify OTP is 4-8 digits
12. Enter OTP (second factor)
13. Submit OTP verification
14. Verify successful MFA authentication and user logged in
15. Perform logout
16. Verify logout successful

**Validates:**
- Complete MFA authentication flow
- Password authentication (first factor)
- OTP page loading after password authentication
- Automatic test skip if MFA not configured
- SMS message capture by mock server
- OTP extraction from SMS (4-8 digit validation)
- OTP verification (second factor)
- Successful login after both factors

**Notes:**
- Test automatically skips if `SAMPLE_APP_URL` is not set
- Test skips with warning if OTP page doesn't load (MFA not configured)
- Mock SMS server runs on port 8098 by default
- Test clears SMS message history before execution
- With automated setup, all prerequisites are handled automatically

### TC002: OTP Required After Password
**Test**: `TC002: Verify OTP is required after password authentication`

**Steps:**
1. Navigate to sample app home page
2. Verify home page loaded
3. Click "Sign In" button
4. Verify login page displayed
5. Enter username and password (first factor)
6. Submit login form
7. Verify OTP verification page is displayed
8. Confirm user is not logged in yet (OTP input visible)

**Validates:**
- Second factor (OTP) is required after password authentication
- Cannot bypass OTP verification
- Authentication incomplete without OTP
- MFA flow enforces second factor

**Notes:**
- Test automatically skips if MFA not configured
- Verifies OTP input is visible, confirming user hasn't proceeded

### TC003: Incorrect OTP Validation
**Test**: `TC003: Verify incorrect OTP shows error`

**Steps:**
1. Navigate to sample app home page
2. Verify home page loaded
3. Click "Sign In" button and complete password authentication
4. Verify OTP verification page displayed
5. Wait for correct OTP to be sent (2 second delay)
6. Retrieve correct OTP from mock SMS server (for logging)
7. Enter incorrect OTP (000000) instead
8. Submit incorrect OTP verification
9. Wait for response (2 second delay)
10. Verify error message displayed OR user remains on OTP page

**Validates:**
- Invalid OTP is rejected by the system
- Error message displayed to user (if implemented)
- User remains on OTP page when OTP is incorrect
- Cannot login with wrong OTP
- System validates OTP correctness

**Notes:**
- Test checks for either error message or remaining on OTP page
- Logs warning if user proceeds (potential security issue)
- Test automatically skips if MFA not configured

## Architecture

### Automated Setup Utility

The `MFASetup` utility (`utils/server-setup/mfa-setup.ts`) automates the complete MFA configuration:

**Setup Process:**
1. **Admin Authentication** - Obtains admin token via flow execution
2. **Notification Sender Creation** - Creates custom SMS sender pointing to mock server
3. **MFA Flow Creation** - Creates complete authentication flow with SMS OTP nodes
4. **Test User Creation** - Creates user with mobile number attribute
5. **Application Update** - Attaches MFA flow to the application

**Cleanup Process:**
- All created resources are automatically deleted after tests complete
- Cleanup functions are registered during setup
- Cleanup runs even if tests fail (in `afterAll` hook)

**Features:**
- Handles existing resources gracefully (e.g., user already exists)
- Provides detailed console logging for debugging
- Can be disabled with `AUTO_SETUP_MFA=false`
- Non-fatal cleanup errors (logged but don't fail tests)

### Mock SMS Server

The tests use a TypeScript-based mock SMS server that:

- **Captures SMS messages** sent by the server
- **Extracts OTP codes** automatically using pattern matching
- **Provides HTTP endpoints** for test access:
  - `POST /send-sms` - Receives SMS from the server
  - `GET /messages` - Retrieve all messages
  - `GET /messages/last` - Get last message
  - `POST /clear` - Clear message history
  - `GET /health` - Health check

**OTP Extraction Logic:**
- Searches for numeric sequences of 4-8 digits
- Prioritizes 6-digit codes (most common)
- Handles multiple SMS message formats

### Page Object Model

**`SampleAppLoginPage`** provides methods for:

**Basic Login:**
- `goto(url)` - Navigate to sample app
- `verifyHomePageLoaded()` - Check home page
- `clickSignInButton()` - Start login flow
- `verifyLoginPageLoaded()` - Verify login form
- `fillLoginForm(username, password)` - Enter credentials
- `clickLogin()` - Submit login
- `verifyLoggedIn()` - Check logged-in state
- `logout()` - Perform logout
- `verifyLoggedOut()` - Verify logout

**MFA/OTP Methods:**
- `verifyOTPPageLoaded()` - Check OTP verification page
- `fillOTP(otp)` - Enter OTP code
- `clickVerifyOTP()` - Submit OTP
- `verifyOTP(otp)` - Complete OTP verification step

### Test Flow Diagram (with Automated Setup)

```
┌─────────────────────────────┐
│   Start Test Suite          │
│ ┌─────────────────────────┐ │
│ │  Start Mock SMS Server  │ │
│ └───────────┬─────────────┘ │
│             │                 │
│ ┌───────────▼─────────────┐ │
│ │  Automated ThunderID Setup│ │
│ │  - Admin Auth           │ │
│ │  - Create Sender        │ │
│ │  - Create MFA Flow      │ │
│ │  - Create Test User     │ │
│ │  - Update Application   │ │
│ └───────────┬─────────────┘ │
└─────────────┼───────────────┘
              │
┌─────────────▼─────────────┐
│   Navigate to App         │
│   Click Sign In           │
└─────────────┬─────────────┘
              │
┌─────────────▼─────────────┐
│   Enter Username/Pass     │
│   (First Factor)          │
└─────────────┬─────────────┘
              │
┌─────────────▼─────────────┐
│   ThunderID Sends SMS       │◄────────┐
│   to Mock Server          │         │
└─────────────┬─────────────┘         │
              │                        │
┌─────────────▼─────────────┐         │
│   Test Retrieves OTP      │         │
│   from Mock Server        │         │
└─────────────┬─────────────┘         │
              │                   Mock SMS
┌─────────────▼─────────────┐     Server
│   Enter OTP Code          │    (Port 8098)
│   (Second Factor)         │         │
└─────────────┬─────────────┘         │
              │                        │
┌─────────────▼─────────────┐         │
│   Verify Logged In        │         │
│   Perform Logout          │         │
└─────────────┬─────────────┘         │
              │                        │
┌─────────────▼─────────────┐         │
│   End Test Suite          │         │
│ ┌─────────────────────────┐         │
│ │  Cleanup ThunderID        │         │
│ │  - Delete Flow          │         │
│ │  - Delete Sender        │         │
│ │  - Delete User          │         │
│ │  Stop Mock SMS Server   │◄────────┘
│ └─────────────────────────┘         │
└───────────────────────────┘
```

## Troubleshooting

### Automated Setup Issues

#### Setup Fails - Application ID Not Found

**Issue**: `SAMPLE_APP_ID not provided - skipping automated setup`

**Solutions**:
1. Ensure `SAMPLE_APP_ID` is set in `.env` file
2. Get application ID from the server:
   ```bash
   curl -k 'https://localhost:8090/applications' \
     -H "Authorization: Bearer <admin-token>"
   ```
3. Or use manual setup with `AUTO_SETUP_MFA=false`

#### Setup Fails - Admin Authentication Error

**Issue**: `Admin authentication failed`

**Solutions**:
1. Verify server is running at `SERVER_URL`
2. Check `ADMIN_USERNAME` and `ADMIN_PASSWORD` are correct
3. Verify application has basic authentication flow configured
4. Check server logs for authentication errors

#### Setup Fails - Resource Creation Error

**Issue**: Failed to create notification sender, flow, or user

**Solutions**:
1. Check server logs for detailed error messages
2. Verify admin user has necessary permissions
3. Check if resources already exist (setup handles this gracefully)
4. Try manual cleanup and re-run:
   ```bash
   # Delete existing resources via API
   # Then re-run tests
   ```

#### Cleanup Warnings

**Issue**: Cleanup errors logged but tests pass

**Explanation**: Cleanup errors are non-fatal and logged as warnings. This is expected behavior if resources were already deleted or don't exist.

**If Persistent**: Manually delete resources via API if needed.

### Tests Are Skipped

**Issue**: Tests show as "skipped" in results

**Solutions**: 
- Ensure `SAMPLE_APP_URL` is set in `.env` file
- Tests automatically skip if URL is not provided
- Verify sample app is running and accessible

### Mock SMS Server Port Conflict

**Issue**: Error starting mock SMS server - port already in use

**Solutions**:
1. Change the port in `.env`:
   ```env
   MOCK_SMS_SERVER_PORT=8099
   ```
2. Kill existing process on port 8098:
   ```bash
   lsof -ti:8098 | xargs kill -9
   ```

### No OTP Received

**Issue**: Test fails because `lastMessage` is null

**Solutions**:
1. Verify notification sender configuration points to `http://localhost:8098/send-sms`
2. Check server logs for SMS sending errors
3. Verify MFA flow has correct `senderId` in OTP nodes
4. Increase wait time: `await page.waitForTimeout(3000);`

### OTP Page Not Loading

**Issue**: Test times out waiting for OTP input page

**Solutions**:
1. Verify MFA flow is configured correctly with OTP prompt
2. Check that password authentication succeeded
3. Verify application is using MFA flow (not basic password-only flow)
4. Check browser console in headed mode: `npm run test:headed`

### Authentication Fails After OTP

**Issue**: Correct OTP entered but authentication fails

**Solutions**:
1. Verify OTP executor has correct `senderId` in verify node
2. Check OTP is being extracted correctly from SMS
3. Verify test user has mobile number attribute
4. Check server logs for OTP verification errors

### Test User Issues

**Issue**: User not found or authentication fails

**Solutions**:
1. Verify test user exists in the server
2. Ensure user has `mobileNumber` attribute
3. Check user is in correct organization unit
4. Verify user credentials in `.env` file

## Best Practices

1. **Always clear messages** between tests to avoid OTP conflicts
2. **Use appropriate wait times** for SMS delivery (1-2 seconds)
3. **Check mock server is running** before executing tests
4. **Verify server configuration** matches test expectations
5. **Use headed mode** for debugging: `npm run test:headed`
6. **Check trace files** for detailed execution: `npm run test:trace`
7. **Review screenshots** in `test-results/` after failures

## Additional Resources

- [MFA Setup Guide](../../../../docs/guides/authentication/GUIDE-SMS-OTP-MFA-LOGIN.md)
- [Sample App Documentation](../../../samples/apps/README.md)
- [Playwright Documentation](https://playwright.dev/docs/intro)
- [Flow Management API](../../../../docs/guides/flows/)

## Support

For issues or questions:
1. Check server logs: `tail -f backend/server.log`
2. Review test trace files: `test-results/*/trace.zip`
3. Run tests in debug mode: `npm run test:debug`
4. Check mock SMS server console output during test execution
