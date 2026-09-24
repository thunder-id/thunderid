# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

"""Unpack, configure and start a ThunderID distribution for OIDC conformance testing.

The conformance suite runs inside Docker and reaches the server through a hostname that
must match both the TLS certificate and the OIDC issuer. ThunderID derives its issuer from
``server.hostname``/``server.port`` in ``deployment.yaml`` and reads the listen address from
the same place, so the hostname cannot be supplied through an environment variable or the
``--port`` flag of ``start.sh`` -- both leave the bound address untouched. This script
therefore rewrites ``deployment.yaml`` before the first start.
"""

import argparse
import os
import shutil
import ssl
import subprocess
import sys
import time
import zipfile

import httpx

from common import suite_callback, verify_for
import yaml

LIVENESS_PATH = "/health/liveness"
STARTUP_TIMEOUT_SECONDS = 180
POLL_INTERVAL_SECONDS = 2


def check_tls_support():
    """Fail early if the interpreter cannot speak TLS 1.3.

    ThunderID sets ``tls.min_version: "1.3"``. Pythons linked against LibreSSL -- notably the
    system Python shipped with macOS -- top out at TLS 1.2, so every request here would fail
    the handshake and look like a server that never came up. Say so plainly instead.
    """
    try:
        context = ssl.create_default_context()
        context.minimum_version = ssl.TLSVersion.TLSv1_3
    except (ValueError, AttributeError):
        sys.exit(
            f"ERROR: this Python cannot negotiate TLS 1.3 (linked against {ssl.OPENSSL_VERSION}), "
            "but the server requires it.\n"
            "       Use a Python built against OpenSSL 1.1.1+ (e.g. Homebrew python@3.12)."
        )


def extract_distribution(zip_path, target_dir):
    """Extract the distribution zip and return the path to the unpacked product home."""
    if not os.path.isfile(zip_path):
        sys.exit(f"ERROR: distribution zip not found: {zip_path}")

    os.makedirs(target_dir, exist_ok=True)
    with zipfile.ZipFile(zip_path) as archive:
        archive.extractall(target_dir)
        roots = {name.split("/")[0] for name in archive.namelist() if "/" in name}

    if len(roots) != 1:
        sys.exit(f"ERROR: expected exactly one top level directory in the zip, found: {sorted(roots)}")

    product_home = os.path.join(target_dir, roots.pop())
    print(f">>> Extracted distribution to: {product_home}")

    # The zip does not preserve the executable bit on every platform.
    for script in ("setup.sh", "start.sh", "thunderid"):
        script_path = os.path.join(product_home, script)
        if os.path.isfile(script_path):
            os.chmod(script_path, 0o755)

    return product_home


def configure_deployment(product_home, hostname, port, bind_address):
    """Point the server at the conformance hostname/port.

    The bind address and the advertised URL have to be set separately. ThunderID binds to
    ``server.hostname``, and a conformance hostname normally resolves to 127.0.0.1, which
    would leave the server on loopback and unreachable from the suite's containers. So bind
    ``0.0.0.0`` and use ``server.public_url`` for the identity the outside world sees.

    ``jwt.issuer`` is left unset on purpose: ThunderID derives it from ``public_url`` when
    that is set, which keeps the issuer, the discovery document and the token ``iss`` claim
    all consistent with the URL the suite actually dials.
    """
    deployment_path = os.path.join(product_home, "deployment.yaml")
    with open(deployment_path, encoding="utf-8") as handle:
        config = yaml.safe_load(handle)

    config.setdefault("server", {})
    config["server"]["hostname"] = bind_address
    config["server"]["port"] = port
    config["server"]["public_url"] = f"https://{hostname}:{port}"

    # Passkey origins are validated against the browser origin, which changes with the host.
    config.setdefault("passkey", {})["allowed_origins"] = [f"https://{hostname}:{port}"]

    # The conformance suite registers its two relying parties anonymously over DCR. ThunderID
    # ships with dcr.insecure=false, which requires an authenticated caller holding the right
    # permission, so registration is refused with unauthorized_client and every module fails
    # before it starts. Open it up for this throwaway server only -- never in a real
    # deployment, which is why it is set here rather than in the shipped defaults.
    oauth = config.setdefault("oauth", {})
    dcr = oauth.setdefault("dcr", {})
    dcr["enabled"] = True
    dcr["insecure"] = True

    with open(deployment_path, "w", encoding="utf-8") as handle:
        yaml.safe_dump(config, handle, default_flow_style=False, sort_keys=False)

    print(f">>> Configured server at https://{hostname}:{port}")
    return f"https://{hostname}:{port}"


def generate_certificate(product_home, hostname):
    """Replace the generated self-signed certificate with one valid for ``hostname``.

    ``setup.sh`` issues a certificate for ``CN=localhost`` only. The conformance suite
    verifies the hostname it dialled, so a certificate without the conformance hostname in
    its SAN list fails every test at the TLS handshake.
    """
    cert_dir = os.path.join(product_home, "config", "certs")
    os.makedirs(cert_dir, exist_ok=True)

    subprocess.run(
        [
            "openssl", "req", "-x509", "-nodes", "-days", "365", "-newkey", "rsa:2048",
            "-keyout", os.path.join(cert_dir, "server.key"),
            "-out", os.path.join(cert_dir, "server.cert"),
            "-subj", f"/O=ThunderID/OU=ThunderID/CN={hostname}",
            "-addext", f"subjectAltName=DNS:{hostname},DNS:localhost,IP:127.0.0.1",
        ],
        check=True,
    )
    print(f">>> Generated server certificate for {hostname}")
    return os.path.join(cert_dir, "server.cert")


def run_setup(product_home, admin_username, admin_password):
    """Run first-time setup: crypto keys, database seeding and default resources."""
    print(">>> Running first time setup ...")
    subprocess.run(
        ["./setup.sh", "--admin-username", admin_username, "--admin-password", admin_password],
        cwd=product_home,
        check=True,
    )


def start_server(product_home, log_path):
    """Start the server detached, streaming its output to ``log_path``."""
    print(">>> Starting ThunderID server ...")
    log_file = open(log_path, "w", encoding="utf-8")  # noqa: SIM115 - handed to the child process
    process = subprocess.Popen(
        ["./start.sh"],
        cwd=product_home,
        stdout=log_file,
        stderr=subprocess.STDOUT,
        start_new_session=True,
    )
    return process


def wait_for_server(base_url, process, log_path):
    """Block until the liveness endpoint responds or the startup budget is exhausted."""
    deadline = time.time() + STARTUP_TIMEOUT_SECONDS
    url = base_url + LIVENESS_PATH

    while time.time() < deadline:
        if process.poll() is not None:
            _dump_log(log_path)
            sys.exit(f"ERROR: server exited during startup with code {process.returncode}")
        try:
            response = httpx.get(url, verify=verify_for(url), timeout=5)
            if response.status_code == 200:
                print(f">>> Server is live at {base_url}")
                return
        except httpx.HTTPError:
            pass
        time.sleep(POLL_INTERVAL_SECONDS)

    _dump_log(log_path)
    sys.exit(f"ERROR: server did not become live within {STARTUP_TIMEOUT_SECONDS}s")


def verify_discovery(base_url):
    """Fetch the discovery document so a misconfigured issuer fails here, not mid-plan."""
    url = f"{base_url}/.well-known/openid-configuration"
    response = httpx.get(url, verify=verify_for(url), timeout=10)
    response.raise_for_status()
    metadata = response.json()

    issuer = metadata.get("issuer")
    if issuer != base_url:
        sys.exit(f"ERROR: issuer '{issuer}' does not match the expected base URL '{base_url}'")

    _check_dcr_is_open(metadata)

    print(f">>> Discovery OK. issuer={issuer}")
    print(f"    response_types_supported={metadata.get('response_types_supported')}")
    print(f"    code_challenge_methods_supported={metadata.get('code_challenge_methods_supported')}")
    print(f"    registration_endpoint={metadata.get('registration_endpoint')}")
    return metadata


def _check_dcr_is_open(metadata):
    """Confirm anonymous client registration works before the plan depends on it.

    The suite registers its relying parties with no credentials. If registration is closed
    every module dies at its first step with the same ``unauthorized_client``, which reads
    as a plan full of unrelated conformance failures rather than one setting.
    """
    endpoint = metadata.get("registration_endpoint")
    if not endpoint:
        sys.exit("ERROR: the server does not advertise a registration_endpoint")

    probe = {
        "client_name": "conformance-preflight",
        "redirect_uris": [suite_callback("preflight")],
        "grant_types": ["authorization_code"],
        "response_types": ["code"],
        "token_endpoint_auth_method": "client_secret_basic",
    }
    response = httpx.post(endpoint, json=probe, verify=verify_for(endpoint), timeout=30)

    if response.status_code != 201:
        sys.exit(
            f"ERROR: anonymous dynamic client registration failed "
            f"({response.status_code}): {response.text.strip()}\n"
            "       The conformance suite registers its clients with no credentials, so "
            "oauth.dcr.insecure must be true on the server under test."
        )

    print(">>> Dynamic client registration accepts anonymous clients.")


def _dump_log(log_path):
    if os.path.isfile(log_path):
        print("=========== server log ===========")
        with open(log_path, encoding="utf-8") as handle:
            print(handle.read())
        print("==================================")


def main():
    parser = argparse.ArgumentParser(description="Configure and start ThunderID for conformance testing.")
    parser.add_argument("zip_path", help="Path to the ThunderID distribution zip.")
    parser.add_argument("work_dir", help="Directory to extract the distribution into.")
    parser.add_argument("--hostname", default="localhost", help="Hostname the suite will use.")
    parser.add_argument("--port", type=int, default=8090, help="Port to bind the server to.")
    parser.add_argument(
        "--bind-address",
        default="0.0.0.0",
        help="Interface to listen on. Defaults to all, so the suite's containers can connect.",
    )
    parser.add_argument("--admin-username", default="admin")
    parser.add_argument("--admin-password", default="admin")
    parser.add_argument("--log-file", default="thunderid-server-log.txt")
    parser.add_argument("--cert-out", help="Copy the generated server certificate here.")
    args = parser.parse_args()

    check_tls_support()
    product_home = extract_distribution(args.zip_path, args.work_dir)
    base_url = configure_deployment(product_home, args.hostname, args.port, args.bind_address)
    run_setup(product_home, args.admin_username, args.admin_password)

    # setup.sh generates a localhost-only certificate, so re-issue afterwards.
    cert_path = generate_certificate(product_home, args.hostname)
    if args.cert_out:
        shutil.copyfile(cert_path, args.cert_out)
        print(f">>> Copied server certificate to {args.cert_out}")

    log_path = os.path.abspath(args.log_file)
    process = start_server(product_home, log_path)
    wait_for_server(base_url, process, log_path)
    verify_discovery(base_url)

    if github_env := os.environ.get("GITHUB_ENV"):
        with open(github_env, "a", encoding="utf-8") as handle:
            handle.write(f"THUNDERID_HOME={product_home}\n")
            handle.write(f"THUNDERID_BASE_URL={base_url}\n")

    print(">>> ThunderID is ready for conformance testing.")


if __name__ == "__main__":
    main()
