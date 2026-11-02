# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

"""Patch the conformance suite's compose file so its containers can reach and trust the server.

Three changes are needed:

* Mongo's ``./mongo/data`` bind mount becomes a named volume, so the container does not fail
  to chown a host directory and does not leave a root-owned one inside the workspace.

* ``extra_hosts`` maps the conformance hostname to the runner, since the suite's containers
  cannot otherwise resolve it.
* A truststore holding the server certificate is mounted and pointed at via JVM flags. The
  suite's backend HTTP client trusts any certificate, but its browser automation runs on
  HtmlUnit, which does not, so the login page would otherwise fail the TLS handshake.
"""

import argparse

import yaml

TRUSTSTORE_MOUNT = "/truststore/thunderid-truststore.jks"
MONGO_VOLUME = "mongo-data"


def patch(compose_path, host, ip, truststore_path, storepass):
    with open(compose_path, encoding="utf-8") as handle:
        compose = yaml.safe_load(handle)

    services = compose.get("services", {})

    # Mongo bind-mounts ./mongo/data from the host by default. The image runs as uid 999 and
    # cannot chown a directory owned by the calling user, so it exits and the Java server dies
    # after it with UnknownHostException: mongodb. On CI the directory it leaves behind is
    # root-owned and inside the workspace, which later breaks any step that walks the tree
    # (actions/cache's hashFiles fails to hash the workspace at job cleanup). A named volume
    # avoids both problems.
    mongodb = services.get("mongodb")
    if mongodb is not None:
        converted = []
        for mount in mongodb.get("volumes", []):
            if isinstance(mount, str) and mount.split(":")[0].endswith("mongo/data"):
                target = mount.split(":", 1)[1]
                converted.append(f"{MONGO_VOLUME}:{target}")
            else:
                converted.append(mount)
        if converted != mongodb.get("volumes", []):
            print(">>> Converted the mongo bind mount to the named volume")
        mongodb["volumes"] = converted
        compose.setdefault("volumes", {}).setdefault(MONGO_VOLUME, {})

    # The browser and the backend HTTP client both live in the server container.
    server = services.get("server")
    if server is None:
        raise SystemExit(f"ERROR: no 'server' service found in {compose_path}")

    server.setdefault("extra_hosts", [])
    entry = f"{host}:{ip}"
    if entry not in server["extra_hosts"]:
        server["extra_hosts"].append(entry)

    # Dedupe on the container target, not the whole entry: a previous run leaves a mount
    # with a different host path but the same target, and appending a second one there is
    # a silent conflict that serves whichever the runtime picks. Replace it in place.
    server.setdefault("volumes", [])
    mount = f"{truststore_path}:{TRUSTSTORE_MOUNT}:ro"
    kept = [
        existing
        for existing in server["volumes"]
        if f":{TRUSTSTORE_MOUNT}:" not in existing
        and not existing.endswith(f":{TRUSTSTORE_MOUNT}")
    ]
    dropped = len(server["volumes"]) - len(kept)
    kept.append(mount)
    server["volumes"] = kept

    command = server.get("command")
    if not isinstance(command, str):
        raise SystemExit("ERROR: expected the server service to use a string command")

    # The JVM only reads these as -D flags before -jar, so splice them in ahead of it.
    flags = (
        f"-Djavax.net.ssl.trustStore={TRUSTSTORE_MOUNT} "
        f"-Djavax.net.ssl.trustStorePassword={storepass} "
    )
    if "-Djavax.net.ssl.trustStore=" not in command:
        command = command.replace("-jar", flags + "-jar", 1)
        server["command"] = command

    # A named volume declared with no options parses as None and safe_dump writes it back
    # as "mongo-data: null", which docker compose rejects as an invalid volume. An empty
    # mapping is the equivalent form it does accept.
    for name, definition in list(compose.get("volumes", {}).items()):
        if definition is None:
            compose["volumes"][name] = {}

    with open(compose_path, "w", encoding="utf-8") as handle:
        yaml.safe_dump(compose, handle, default_flow_style=False, sort_keys=False)

    print(f">>> Mapped {entry} for the server container")
    if dropped:
        print(f">>> Replaced {dropped} stale truststore mount(s)")
    print(f">>> Mounted truststore at {TRUSTSTORE_MOUNT}")


def main():
    parser = argparse.ArgumentParser(description="Patch the conformance suite compose file.")
    parser.add_argument("compose_file")
    parser.add_argument("--host", required=True)
    parser.add_argument("--ip", required=True)
    parser.add_argument("--truststore", required=True)
    parser.add_argument("--storepass", default="changeit")
    args = parser.parse_args()

    patch(args.compose_file, args.host, args.ip, args.truststore, args.storepass)


if __name__ == "__main__":
    main()
