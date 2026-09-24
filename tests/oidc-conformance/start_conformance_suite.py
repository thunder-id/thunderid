# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

"""Bring up the OpenID conformance suite via Docker Compose and wait until it is serving."""

import argparse
import subprocess
import sys
import time

import httpx

from common import SUITE_URL, verify_for

STARTUP_TIMEOUT_SECONDS = 300
POLL_INTERVAL_SECONDS = 5


def start_suite(compose_file):
    print(">>> Starting conformance suite ...")
    subprocess.run(
        ["docker", "compose", "-f", compose_file, "up", "-d"],
        check=True,
    )


def wait_for_suite(compose_file):
    deadline = time.time() + STARTUP_TIMEOUT_SECONDS
    url = f"{SUITE_URL}/api/runner/available"

    while time.time() < deadline:
        try:
            response = httpx.get(url, verify=verify_for(url), timeout=10)
            if response.status_code == 200:
                print(f">>> Conformance suite is available at {SUITE_URL}")
                return
        except httpx.HTTPError:
            pass
        time.sleep(POLL_INTERVAL_SECONDS)

    print("ERROR: conformance suite did not start in time. Container logs follow:")
    subprocess.run(["docker", "compose", "-f", compose_file, "logs", "--tail", "200"], check=False)
    sys.exit(1)


def main():
    parser = argparse.ArgumentParser(description="Start the OpenID conformance suite.")
    parser.add_argument("compose_file", help="Path to the suite's docker compose file.")
    args = parser.parse_args()

    start_suite(args.compose_file)
    wait_for_suite(args.compose_file)


if __name__ == "__main__":
    main()
