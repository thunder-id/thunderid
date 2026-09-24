# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

"""Provision the user account the conformance suite authenticates as.

Obtains an admin access token with the same authorization-code + PKCE dance the e2e harness
uses (see ``tests/e2e/run-e2e.sh``), then creates a user with the profile claims the OIDC
Basic profile asserts on: ``name``, ``given_name``, ``family_name`` and ``email``.
"""

import argparse
import base64
import hashlib
import json
import os
import secrets
import sys
import urllib.parse

import httpx

from common import verify_for

CONSOLE_CLIENT_ID = "CONSOLE"

# The bootstrap registers the system resource server with this literal identifier, regardless of
# the hostname the server is configured on (see bootstrap/01-default-resources.yaml). A token
# request must echo it exactly: there is no default resource server, so omitting `resource`
# fails with invalid_target, and deriving it from the base URL fails the same way whenever the
# conformance run uses a non-default hostname.
SYSTEM_RESOURCE_IDENTIFIER = "https://localhost:8090/mcp"


def pkce_pair():
    verifier = secrets.token_hex(32)[:43]
    digest = hashlib.sha256(verifier.encode("ascii")).digest()
    challenge = base64.urlsafe_b64encode(digest).decode("ascii").rstrip("=")
    return verifier, challenge


def get_admin_token(http, base_url, username, password):
    """Run the console login flow end to end and return an admin access token."""
    verifier, challenge = pkce_pair()
    redirect_uri = f"{base_url}/console"

    authorize = http.get(
        f"{base_url}/oauth2/authorize",
        params={
            "client_id": CONSOLE_CLIENT_ID,
            "redirect_uri": redirect_uri,
            "scope": "system",
            "resource": SYSTEM_RESOURCE_IDENTIFIER,
            "response_type": "code",
            "code_challenge": challenge,
            "code_challenge_method": "S256",
        },
        follow_redirects=False,
    )

    location = authorize.headers.get("location")
    if not location:
        sys.exit(f"ERROR: authorize did not redirect. status={authorize.status_code}")

    query = urllib.parse.parse_qs(urllib.parse.urlparse(location).query)
    auth_id = query.get("authId", [""])[0]
    execution_id = query.get("executionId", [""])[0]
    if not auth_id or not execution_id:
        sys.exit(f"ERROR: could not parse authId/executionId from: {location}")

    # The console flow runs an SSO check first. This is a cookie-less login, so the first call
    # advances past that check to the credentials prompt and mints a challenge token.
    prompt = http.post(f"{base_url}/flow/execute", json={"executionId": execution_id}).json()
    challenge_token = prompt.get("challengeToken")
    if not challenge_token:
        sys.exit(f"ERROR: flow did not return a challenge token: {prompt}")

    flow = http.post(
        f"{base_url}/flow/execute",
        json={
            "executionId": execution_id,
            "challengeToken": challenge_token,
            "inputs": {"username": username, "password": password},
            "action": "action_001",
        },
    ).json()
    assertion = flow.get("assertion")
    if not assertion:
        sys.exit(f"ERROR: flow did not return an assertion: {flow}")

    callback = http.post(
        f"{base_url}/oauth2/auth/callback",
        json={"authId": auth_id, "assertion": assertion},
    ).json()
    redirect = callback.get("redirect_uri", "")
    code = urllib.parse.parse_qs(urllib.parse.urlparse(redirect).query).get("code", [""])[0]
    if not code:
        sys.exit(f"ERROR: callback did not return an authorization code: {callback}")

    token = http.post(
        f"{base_url}/oauth2/token",
        data={
            "grant_type": "authorization_code",
            "code": code,
            "redirect_uri": redirect_uri,
            "client_id": CONSOLE_CLIENT_ID,
            "resource": SYSTEM_RESOURCE_IDENTIFIER,
            "code_verifier": verifier,
        },
        headers={"Content-Type": "application/x-www-form-urlencoded"},
    ).json()

    access_token = token.get("access_token")
    if not access_token:
        sys.exit(f"ERROR: token request failed: {token}")

    print(">>> Obtained admin access token")
    return access_token


def resolve_person_ou(http, base_url, headers):
    """Find the organisational unit id that the built-in Person user type belongs to."""
    response = http.get(f"{base_url}/user-types", headers=headers)
    response.raise_for_status()
    payload = response.json()

    user_types = payload.get("types", payload if isinstance(payload, list) else [])
    for user_type in user_types:
        if user_type.get("name") == "Person" or user_type.get("handle") == "Person":
            ou_id = user_type.get("ouId")
            if ou_id:
                return ou_id

    sys.exit(f"ERROR: could not resolve the OU for the Person user type from: {json.dumps(payload)[:500]}")


def create_user(http, base_url, headers, ou_id, username, password, email):
    """Create the conformance user, tolerating a pre-existing account on re-runs."""
    existing = http.get(
        f"{base_url}/users",
        headers=headers,
        params={"filter": f'username eq "{username}"'},
    )
    if existing.status_code == 200:
        users = existing.json().get("users", [])
        if users:
            print(f">>> User '{username}' already exists (id={users[0].get('id')})")
            return users[0].get("id")

    response = http.post(
        f"{base_url}/users",
        headers=headers,
        json={
            "type": "Person",
            "ouId": ou_id,
            # Kept in step with the built-in Person schema, which is strict: unknown keys are
            # rejected. These are the claims the Basic profile's UserInfo assertions read.
            "attributes": {
                "username": username,
                "password": password,
                "sub": username,
                "email": email,
                "name": "Conformance Test User",
                "given_name": "Conformance",
                "family_name": "User",
                "mobile_number": "+12345678920",
            },
        },
    )

    if response.status_code not in (200, 201):
        sys.exit(f"ERROR: user creation failed ({response.status_code}): {response.text}")

    user_id = response.json().get("id")
    print(f">>> Created conformance user '{username}' (id={user_id})")
    return user_id


def main():
    parser = argparse.ArgumentParser(description="Create the OIDC conformance test user.")
    parser.add_argument("--base-url", required=True)
    parser.add_argument("--admin-username", default="admin")
    parser.add_argument("--admin-password", default="admin")
    parser.add_argument("--username", default="conformance-user")
    parser.add_argument("--password", default="Conformance@123")
    parser.add_argument("--email", default="conformance-user@thunderid.test")
    args = parser.parse_args()

    with httpx.Client(verify=verify_for(args.base_url), timeout=30) as http:
        token = get_admin_token(http, args.base_url, args.admin_username, args.admin_password)
        headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}
        ou_id = resolve_person_ou(http, args.base_url, headers)
        create_user(http, args.base_url, headers, ou_id, args.username, args.password, args.email)

    if github_env := os.environ.get("GITHUB_ENV"):
        with open(github_env, "a", encoding="utf-8") as handle:
            handle.write(f"CONFORMANCE_USERNAME={args.username}\n")

    print(">>> Conformance user is ready.")


if __name__ == "__main__":
    main()
