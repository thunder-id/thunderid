# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

"""Provision the static clients an OIDC conformance plan runs against.

The suite's `client_registration=static_client` variant expects client credentials to already
exist in the plan config, rather than registering its own relying parties mid-run. Create the
clients once up front, read their credentials back, and hand them to the suite in the config
file.

Registration still goes through DCR here because that is the only client-creation API
ThunderID exposes. The distinction that matters to the suite is not how the client was born,
but that it is not registering (and deleting) clients itself during the run, which is what
makes a registration access token unnecessary.

DCR carries no flow selection, so a registered client runs the deployment's default
authentication flow. That flow does not check for an existing SSO session, which leaves
prompt=none, id_token_hint and max_age unanswerable. The clients are therefore rebound after
registration to a flow that does check, created here from sso-flow.json.
"""

import argparse
import json
import os
import secrets
import sys

import httpx

from setup_test_user import get_admin_token
from common import suite_callback, verify_for

# Every client needs the grants and response types the basic profile exercises.
CLIENT_TEMPLATE = {
    "grant_types": ["authorization_code", "refresh_token"],
    "response_types": ["code"],
    "token_endpoint_auth_method": "client_secret_basic",
    "scope": "openid profile email address phone",
}


def discover(base_url):
    url = f"{base_url.rstrip('/')}/.well-known/openid-configuration"
    response = httpx.get(url, verify=verify_for(url), timeout=30)
    response.raise_for_status()
    return response.json()


def register_client(endpoint, name, alias, auth_method="client_secret_basic"):
    """Register one client and return the credentials the suite needs.

    Every module in a plan shares one alias, so every client redirects to the same callback.
    """
    body = dict(CLIENT_TEMPLATE)
    body["client_name"] = name
    body["token_endpoint_auth_method"] = auth_method
    body["redirect_uris"] = [suite_callback(alias)]

    response = httpx.post(endpoint, json=body, verify=verify_for(endpoint), timeout=30)
    if response.status_code != 201:
        sys.exit(
            f"ERROR: could not register '{name}' ({response.status_code}): "
            f"{response.text.strip()}"
        )

    registered = response.json()
    client_id = registered.get("client_id")
    client_secret = registered.get("client_secret")
    if not client_id or not client_secret:
        sys.exit(
            f"ERROR: registration of '{name}' returned no usable credentials: "
            f"{json.dumps(registered)}"
        )

    print(f">>> Registered {name}: client_id={client_id}")
    return {"client_id": client_id, "client_secret": client_secret}


def ensure_sso_flow(http, base_url, flow_path):
    """Create the SSO-aware authentication flow, returning its id.

    The flow is idempotent by handle: a run against a server that already has it reuses the
    existing one rather than failing, so the harness can be re-run without cleanup.
    """
    with open(flow_path, encoding="utf-8") as handle:
        definition = json.load(handle)

    flows = http.get(f"{base_url}/flows", params={"limit": 200})
    flows.raise_for_status()
    payload = flows.json()
    existing = payload.get("flows", payload) if isinstance(payload, dict) else payload
    for flow in existing or []:
        if flow.get("handle") == definition["handle"]:
            print(f">>> Reusing existing flow '{definition['handle']}' (id={flow['id']})")
            return flow["id"]

    created = http.post(f"{base_url}/flows", json=definition)
    if created.status_code not in (200, 201):
        sys.exit(
            f"ERROR: could not create the SSO flow ({created.status_code}): "
            f"{created.text.strip()}"
        )
    flow_id = created.json()["id"]
    print(f">>> Created flow '{definition['handle']}' (id={flow_id})")
    return flow_id


def bind_flow(http, base_url, client_id, flow_id):
    """Point the application backing ``client_id`` at ``flow_id``.

    DCR has no flow field, so the client is registered first and rebound here. The update is a
    PUT, so the application is read back and resent with only authFlowId changed.
    """
    found = http.get(f"{base_url}/applications", params={"limit": 200})
    found.raise_for_status()
    payload = found.json()
    apps = payload.get("applications", payload) if isinstance(payload, dict) else payload

    app_id = None
    for app in apps or []:
        if app.get("clientId") == client_id:
            app_id = app["id"]
            break
    if not app_id:
        sys.exit(f"ERROR: no application found for client_id {client_id}")

    current = http.get(f"{base_url}/applications/{app_id}")
    current.raise_for_status()
    body = current.json()

    if body.get("authFlowId") == flow_id:
        return

    body["authFlowId"] = flow_id
    updated = http.put(f"{base_url}/applications/{app_id}", json=body)
    if updated.status_code not in (200, 204):
        sys.exit(
            f"ERROR: could not bind the flow to {client_id} ({updated.status_code}): "
            f"{updated.text.strip()}"
        )


def main():
    parser = argparse.ArgumentParser(
        description="Register the static clients a conformance plan runs against."
    )
    parser.add_argument("--base-url", required=True, help="ThunderID base URL, e.g. https://localhost:8090")
    parser.add_argument(
        "--alias",
        help="Plan alias. Defaults to the alias in --config, which is where the plan reads it from.",
    )
    parser.add_argument(
        "--config",
        default=os.path.join(os.path.dirname(os.path.abspath(__file__)), "plan-config.json"),
        help="Plan config to read the alias from when --alias is not given.",
    )
    parser.add_argument(
        "--output",
        required=True,
        help="Where to write the {client, client2} credentials run_tests.py merges in.",
    )
    parser.add_argument("--admin-username", default="admin")
    parser.add_argument("--admin-password", default="admin")
    parser.add_argument(
        "--sso-flow",
        default=os.path.join(os.path.dirname(os.path.abspath(__file__)), "sso-flow.json"),
        help="Authentication flow to bind the clients to, created if the server does not have it.",
    )
    parser.add_argument(
        "--no-sso-flow",
        action="store_true",
        help=(
            "Leave the clients on the deployment's default flow. The modules that turn on "
            "reusing a session will fail unless that flow checks for one."
        ),
    )
    args = parser.parse_args()

    # The callback the suite listens on is built from the plan alias, so it has to be the one
    # the plan itself uses. Read it from the same file rather than repeating it at each caller.
    alias = args.alias
    if not alias:
        with open(args.config, encoding="utf-8") as handle:
            alias = json.load(handle)["alias"]

    metadata = discover(args.base_url)
    endpoint = metadata.get("registration_endpoint")
    if not endpoint:
        sys.exit("ERROR: the server does not advertise a registration_endpoint")

    # Three clients. OIDCCServerTest and OIDCCRefreshToken assert that a token issued to one
    # client is rejected by the other, so client and client2 need distinct registrations.
    # oidcc-server-client-secret-post reads its own client_secret_post block, because a
    # client may only advertise one token endpoint auth method.
    # ThunderID rejects a duplicate client_name, so make the names unique per run. Clients
    # from previous runs stay registered and harmless; the plan only ever uses the ones
    # named in the config file this writes.
    suffix = secrets.token_hex(4)
    clients = {
        "client": register_client(endpoint, f"Conformance Suite RP 1 {suffix}", alias),
        "client2": register_client(endpoint, f"Conformance Suite RP 2 {suffix}", alias),
        "client_secret_post": register_client(
            endpoint,
            f"Conformance Suite RP Secret Post {suffix}",
            alias,
            auth_method="client_secret_post",
        ),
    }

    if not args.no_sso_flow:
        # DCR cannot select a flow, so the clients are rebound now that they exist. Without
        # this they run the default flow, which does not consult an existing session, and
        # prompt=none, id_token_hint and max_age cannot be answered.
        http = httpx.Client(verify=verify_for(args.base_url), timeout=60)
        token = get_admin_token(http, args.base_url, args.admin_username, args.admin_password)
        http.headers["Authorization"] = f"Bearer {token}"

        flow_id = ensure_sso_flow(http, args.base_url, args.sso_flow)
        for key, credentials in clients.items():
            bind_flow(http, args.base_url, credentials["client_id"], flow_id)
            print(f">>> Bound {key} to the SSO flow")

    with open(args.output, "w", encoding="utf-8") as handle:
        json.dump(clients, handle, indent=2)

    print(f">>> Wrote static client credentials to {args.output}")


if __name__ == "__main__":
    main()
