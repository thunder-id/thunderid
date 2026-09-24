# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

"""Values and helpers the conformance scripts share.

Two things were duplicated across the harness and are defined here once: the address of the
conformance suite, and the decision of whether to verify a TLS certificate.
"""

import ipaddress
import socket
import urllib.parse

# The suite advertises itself as localhost.emobix.co.uk (a real hostname that resolves to
# 127.0.0.1) via --fintechlabs.base_url in docker-compose-dev.yml. Dial the same name so its
# certificate matches and the redirect URIs it issues line up with what we call. This address
# also reaches the scripts' callers through plan-config.json and the workflow, so changing it
# here is necessary but not sufficient.
SUITE_HOST = "localhost.emobix.co.uk"
SUITE_PORT = 8443
SUITE_URL = f"https://{SUITE_HOST}:{SUITE_PORT}"

# Every module in a plan shares one alias, and the suite serves each one's callback under it.
SUITE_CALLBACK_TEMPLATE = f"{SUITE_URL}/test/a/{{alias}}/callback"


def suite_callback(alias):
    """Return the redirect URI the suite serves for ``alias``."""
    return SUITE_CALLBACK_TEMPLATE.format(alias=alias)


def is_loopback(host):
    """Whether ``host`` resolves only to loopback addresses.

    Resolution rather than a list of names: the harness runs the server under an arbitrary
    hostname (CI uses thunderid.conformance.test, mapped to 127.0.0.1 in /etc/hosts) so that
    the certificate, the OIDC issuer and the suite's view of it all agree. Matching names
    would miss those and demand verification of a self-signed certificate.
    """
    if not host:
        return False
    try:
        addresses = {info[4][0] for info in socket.getaddrinfo(host, None)}
    except socket.gaierror:
        # Unresolvable, so nothing can be reached at it. Verification is the safe answer.
        return False
    return bool(addresses) and all(
        ipaddress.ip_address(address).is_loopback for address in addresses
    )


def verify_for(url):
    """Return the ``verify`` value to pass httpx for ``url``.

    False for servers on this machine, which serve self-signed certificates with no CA to
    trust them against, and True for everything else. Keying this on the host actually being
    dialled matters because of what these scripts send: setup_test_user.py posts admin
    credentials, and register_static_clients.py receives the static clients' secrets back.
    Neither should cross an unverified connection to a remote host. To trust a private CA,
    set SSL_CERT_FILE or REQUESTS_CA_BUNDLE, which httpx reads by default.
    """
    return not is_loopback(urllib.parse.urlparse(url).hostname)
