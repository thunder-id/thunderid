# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

"""Drive an OIDC conformance test plan against a running ThunderID server.

Creates the test plan through the conformance suite API, runs every module in it, waits for
each to reach a terminal state, and reports the outcome. Exits non-zero if any module fails.
"""

import argparse
import functools
import json
import os
import re
import sys
import time

# Stream progress as it happens. Without this a run that is killed part way through loses
# every line it printed, because stdout is block-buffered when piped to a file or CI log.
print = functools.partial(print, flush=True)  # noqa: A001 - deliberate module-wide override

import httpx

from browser_driver import BrowserDriver
from common import SUITE_URL, verify_for

MODULE_TIMEOUT_SECONDS = 300
POLL_INTERVAL_SECONDS = 3

# The suite's DELETE /api/runner/{id} runs test.stop() via runInBackground and returns 200
# before the module has actually stopped. Every module in a plan shares one alias, and
# createTestAlias refuses to reassign it until the holder reaches FINISHED/INTERRUPTED, so
# starting the next module too eagerly gets a 409 and it never runs. Wait for the stop to land.
CANCEL_TIMEOUT_SECONDS = 60

# A module is done when it reaches one of these states. The suite also uses NOT_YET_CREATED,
# CREATED, CONFIGURED, RUNNING and WAITING while a module is still in flight.
TERMINAL_STATUSES = {"FINISHED", "INTERRUPTED"}

# The smallest valid PNG the suite's upload validator accepts: it requires a data URI of type
# image/png or image/jpeg under 500KB. Content is never asserted on, only its presence, so a
# 1x1 pixel is enough to release a module parked on a screenshot placeholder.
PLACEHOLDER_IMAGE = (
    "data:image/png;base64,"
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
)

# Results the suite considers acceptable for a passing run. REVIEW and WARNING are surfaced in
# the summary but do not fail the build, matching how the suite itself grades certification runs.
# UNKNOWN is deliberately excluded: the suite uses it for "not yet known, probably still
# running", so treating it as a pass would mask a module that never actually finished.
PASSING_RESULTS = {"PASSED", "WARNING", "REVIEW", "SKIPPED"}


class ConformanceClient:
    """Thin wrapper over the conformance suite's REST API."""

    def __init__(self, base_url):
        self.base_url = base_url.rstrip("/")
        self.http = httpx.Client(verify=verify_for(self.base_url), timeout=30)
        # Set by run_module so a failure can have its log fetched afterwards.
        self.last_module_id = None

    def get_image_placeholders(self, module_id):
        """Return the upload tokens of any screenshot placeholders the module is waiting on."""
        response = self.http.get(f"{self.base_url}/api/log/{module_id}/images")
        response.raise_for_status()
        return [entry["upload"] for entry in response.json() if entry.get("upload")]

    def fill_image_placeholder(self, module_id, placeholder):
        """Satisfy one screenshot placeholder so the module can finish.

        Some modules end by asking a human to upload a screenshot as certification evidence
        (for example "the server must ask the user to login a second time"). The assertions
        have already run at that point; the module simply parks in WAITING until every
        placeholder is filled, then fires finished with a REVIEW result. Filling it here is
        what turns an otherwise indefinite wait into a FINISHED module.
        """
        response = self.http.post(
            f"{self.base_url}/api/log/{module_id}/images/{placeholder}",
            content=PLACEHOLDER_IMAGE,
            headers={"Content-Type": "text/plain"},
        )
        response.raise_for_status()

    def check_certification_package(self, plan_id):
        """Ask the suite itself whether the plan is certification-clean.

        This is the same oracle the OpenID Foundation applies: a module counts as failed
        only when it did not reach FINISHED, or its result was FAILED/UNKNOWN. PASSED,
        WARNING, REVIEW and SKIPPED all pass, which is why screenshot-gated modules
        (REVIEW) do not have to be driven by hand to assert conformance.

        Returns (ok, failed_tests). Note the suite publishes the plan and marks it
        immutable when this succeeds, so it must only be called once, after the run.
        """
        response = self.http.post(
            f"{self.base_url}/api/plan/{plan_id}/certificationpackage",
            # An empty part: the field is only required for relying-party tests, but the
            # endpoint is multipart-only, so it has to be present. httpx needs real bytes
            # here, where requests accepts None.
            files={"clientSideData": ("", b"", "application/zip")},
            timeout=120,
        )
        if response.status_code == 200:
            return True, {}

        try:
            body = response.json()
        except ValueError:
            return False, {"error": f"unexpected response {response.status_code}"}

        if response.status_code == 422:
            return False, body.get("failed_tests", {})
        return False, {"error": body.get("error_description", f"status {response.status_code}")}

    def create_plan(self, plan_name, variant, config):
        params = {"planName": plan_name}
        if variant:
            params["variant"] = json.dumps(variant)

        response = self.http.post(f"{self.base_url}/api/plan", params=params, json=config)
        response.raise_for_status()
        return response.json()

    def create_test_module(self, test_name, plan_id, variant=None):
        """Start one module.

        ``variant`` is the resolved per-module variant returned by the plan. It has to be
        passed back: the basic plan fixes client authentication per module (most modules use
        client_secret_basic, one uses client_secret_post), so dropping it would run that
        module with the wrong auth method and fail it spuriously.
        """
        params = {"test": test_name, "plan": plan_id}
        if variant:
            params["variant"] = json.dumps(variant)

        # A 409 means the shared plan alias is still held by a module that has not finished
        # stopping. That is transient, so retry rather than losing the module for the run.
        deadline = time.time() + CANCEL_TIMEOUT_SECONDS
        while True:
            response = self.http.post(f"{self.base_url}/api/runner", params=params)
            if response.status_code != 409 or time.time() >= deadline:
                break
            print("    alias still held, waiting to retry ...")
            time.sleep(POLL_INTERVAL_SECONDS)

        response.raise_for_status()
        return response.json()["id"]

    def get_module_info(self, module_id):
        response = self.http.get(f"{self.base_url}/api/info/{module_id}")
        response.raise_for_status()
        return response.json()

    def get_plan_info(self, plan_id):
        response = self.http.get(f"{self.base_url}/api/plan/{plan_id}")
        response.raise_for_status()
        return response.json()

    def get_log(self, module_id):
        response = self.http.get(f"{self.base_url}/api/log/{module_id}")
        response.raise_for_status()
        return response.json()

    def get_plan_variants(self, plan_name):
        """Return the variant keys the named plan declares as selectable.

        Plans differ: the basic plan asks the caller to choose server_metadata and
        client_registration, while the config plan declares none and pins them per module.
        Sending a variant a plan already fixes is a 400, so only send what it asks for.
        """
        response = self.http.get(f"{self.base_url}/api/plan/available")
        response.raise_for_status()
        for plan in response.json():
            if plan.get("planName") == plan_name:
                return set(plan.get("variants") or {})
        return set()

    def get_browser_status(self, module_id):
        """Return the module's front-channel state, or None while it has no browser yet."""
        try:
            response = self.http.get(f"{self.base_url}/api/runner/browser/{module_id}")
            response.raise_for_status()
            return response.json()
        except httpx.HTTPError:
            # 404 until the module is running, 503 until it has a BrowserControl.
            return None

    def mark_url_visited(self, module_id, url):
        """Tell the suite the URL has been visited so the module stops waiting on it."""
        try:
            response = self.http.post(
                f"{self.base_url}/api/runner/browser/{module_id}/visit", params={"url": url}
            )
            response.raise_for_status()
        except httpx.HTTPError as error:
            print(f"    warning: could not mark {url} visited: {error}")

    def cancel_module(self, module_id):
        """Stop a module that is still running.

        Modules in a plan share one alias, and starting a second while the first is still
        live makes the suite kill the older one with an "alias conflict". A module abandoned
        at timeout would therefore corrupt whichever module runs next, so retire it here.
        """
        try:
            self.http.delete(f"{self.base_url}/api/runner/{module_id}")
        except httpx.HTTPError as error:
            print(f"    warning: could not cancel module {module_id}: {error}")
            return

        # The DELETE is asynchronous, so poll until the module actually releases the alias.
        deadline = time.time() + CANCEL_TIMEOUT_SECONDS
        while time.time() < deadline:
            try:
                status = self.get_module_info(module_id).get("status", "UNKNOWN")
            except httpx.HTTPError:
                time.sleep(POLL_INTERVAL_SECONDS)
                continue

            if status in TERMINAL_STATUSES:
                print(f"    cancelled (status={status})")
                return

            time.sleep(POLL_INTERVAL_SECONDS)

        print(
            f"    warning: module {module_id} did not stop within "
            f"{CANCEL_TIMEOUT_SECONDS}s; the next module may hit an alias conflict"
        )


# Keys plan-config.json carries for the harness rather than for the suite. The rest of the file
# is posted to the suite verbatim, and it rejects a config holding fields it does not define,
# so these are stripped before the plan is created.
HARNESS_CONFIG_KEYS = ("profiles", "defaultProfile")


def load_profiles(config_path):
    """Return (profiles, default profile name) from the plan config."""
    with open(config_path, encoding="utf-8") as handle:
        config = json.load(handle)
    profiles = config.get("profiles") or {}
    if not profiles:
        sys.exit(f"ERROR: {config_path} declares no profiles")
    return profiles, config.get("defaultProfile") or next(iter(profiles))


def load_config(config_path, base_url, static_clients_path=None):
    """Read the plan config template and substitute the runtime placeholders."""
    with open(config_path, encoding="utf-8") as handle:
        raw = handle.read()

    host = base_url.split("://", 1)[1].split(":")[0]
    raw = raw.replace("https://THUNDERID_HOST:8090", base_url)
    raw = raw.replace("THUNDERID_HOST", host)
    config = json.loads(raw)

    if static_clients_path:
        # The static_client variant reads credentials straight out of the config instead of
        # registering its own clients, so merge in what register_static_clients.py provisioned.
        # Merge rather than replace: the template's client_name and scope still apply.
        with open(static_clients_path, encoding="utf-8") as handle:
            provisioned = json.load(handle)
        for key, credentials in provisioned.items():
            config.setdefault(key, {}).update(credentials)

    for key in HARNESS_CONFIG_KEYS:
        config.pop(key, None)

    return config


def run_module(client, test_name, plan_id, variant=None, driver=None):
    """Run one test module and return (result, status).

    When ``driver`` is set, front-channel URLs the module publishes are visited in a real
    browser between polls. See browser_driver.py for why the suite's own browser cannot do it.
    """
    print(f"\n>>> Running module: {test_name}")
    client.last_module_id = None
    try:
        module_id = client.create_test_module(test_name, plan_id, variant)
    except httpx.HTTPError as error:
        print(f"    ERROR: could not start module: {error}")
        return "FAILED", "NOT_STARTED"

    # The caller needs this to fetch the log of a module that failed. It is kept here rather
    # than returned so the (result, status) shape the outcomes dict is built from stays put.
    client.last_module_id = module_id

    deadline = time.time() + MODULE_TIMEOUT_SECONDS
    status = "CREATED"
    result = "UNKNOWN"
    visited_urls = set()
    filled_placeholders = set()

    try:
        while time.time() < deadline:
            try:
                info = client.get_module_info(module_id)
            except httpx.HTTPError:
                time.sleep(POLL_INTERVAL_SECONDS)
                continue

            status = info.get("status", "UNKNOWN")
            result = info.get("result", "UNKNOWN")

            if status in TERMINAL_STATUSES:
                print(f"    status={status} result={result}")
                return result, status

            if driver is not None:
                driver.drive_pending(module_id, visited_urls)

            # A module parked in WAITING may be waiting on a screenshot upload rather than on
            # anything the browser can do. Its assertions have already run, so fill the
            # placeholder and let it finish instead of holding the run until the timeout.
            if status == "WAITING":
                try:
                    for placeholder in client.get_image_placeholders(module_id):
                        if placeholder in filled_placeholders:
                            continue
                        client.fill_image_placeholder(module_id, placeholder)
                        filled_placeholders.add(placeholder)
                        print(f"    supplied screenshot evidence for placeholder {placeholder}")
                except httpx.HTTPError as error:
                    print(f"    warning: could not fill screenshot placeholder: {error}")

            time.sleep(POLL_INTERVAL_SECONDS)

        # The module may have reached a terminal state between the last poll and here, in
        # which case its real result stands rather than a timeout.
        try:
            info = client.get_module_info(module_id)
            if info.get("status") in TERMINAL_STATUSES:
                status = info.get("status")
                result = info.get("result", "UNKNOWN")
                print(f"    status={status} result={result} (finished as the timeout elapsed)")
                return result, status
        except httpx.HTTPError:
            pass

        print(f"    TIMEOUT after {MODULE_TIMEOUT_SECONDS}s (last status={status})")
        client.cancel_module(module_id)
        return "TIMED_OUT", status
    finally:
        # The module's browser session ends with the module, so its cookies do too. Without
        # this a run of 38 modules would hold 38 contexts open at once.
        if driver is not None:
            driver.release(module_id)


# Keys whose values are credentials rather than evidence. The suite logs whole HTTP
# exchanges, including Authorization headers and token responses, and the file this writes is
# kept as a build artifact that anyone with read access can download. Matched case
# insensitively as substrings, so client_secret, Authorization and id_token are all covered.
SENSITIVE_KEY_PARTS = (
    "authorization",
    "secret",
    "password",
    "token",
    "cookie",
    "assertion",
    "code_verifier",
)

# Values in a form-encoded or header string that have to be masked in place, since they do not
# arrive as their own JSON key.
SENSITIVE_PATTERN = re.compile(
    r"((?:client_secret|code|access_token|refresh_token|id_token|password|assertion|"
    r"code_verifier)=)[^&\s]+",
    re.IGNORECASE,
)

# The same fields written as JSON rather than form encoding, for a body that did not parse,
# such as one the suite captured truncated.
SENSITIVE_JSON_PATTERN = re.compile(
    r'("(?:[A-Za-z0-9_]*(?:secret|password|token|assertion|code_verifier)|code)"\s*:\s*)'
    r'"[^"]*"',
    re.IGNORECASE,
)

# An Authorization header value, whatever the scheme.
AUTH_HEADER_PATTERN = re.compile(
    r"\b(basic|bearer|dpop)\s+[A-Za-z0-9._~+/=-]+", re.IGNORECASE
)


def redact(value, key=""):
    """Return ``value`` with anything credential bearing replaced by a placeholder.

    Applied to every field before it is written, because the suite's log carries the full
    request and response of each HTTP call the test made, and those include client secrets,
    the admin bearer token and issued tokens. What is left is the diagnostic content, which
    is the point of keeping the log at all.
    """
    if any(part in key.lower() for part in SENSITIVE_KEY_PARTS):
        return "<redacted>"
    if isinstance(value, dict):
        return {k: redact(v, k) for k, v in value.items()}
    if isinstance(value, list):
        return [redact(item, key) for item in value]
    if isinstance(value, str):
        # Bodies arrive as strings holding a whole JSON document, so a token response reaches
        # here as text and none of its keys have been seen as keys. Parse it back when it
        # parses, redact it structurally, and re-serialise.
        stripped = value.strip()
        if stripped.startswith(("{", "[")):
            try:
                return redact(json.loads(stripped))
            except ValueError:
                pass
        masked = SENSITIVE_PATTERN.sub(r"\1<redacted>", value)
        masked = SENSITIVE_JSON_PATTERN.sub(r'\1"<redacted>"', masked)
        return AUTH_HEADER_PATTERN.sub(lambda m: f"{m.group(1)} <redacted>", masked)
    return value


def dump_failure_log(client, test_name, module_id, path):
    """Append the suite's event log for a failed module to ``path``.

    GET /api/log/{id} returns the entries the suite shows on its own results page: one
    document per condition, carrying `result` (FAILURE, WARNING, INFO, SUCCESS or REVIEW),
    `src` (the condition class) and `msg`. Only FAILURE and WARNING are kept, since a passing
    module's INFO entries run to thousands of lines and the interesting ones are already in
    the suite's UI.

    The suite is torn down when the job ends, so the plan URL in the results file points at
    nothing by the time anyone reads the notification. This is the copy that survives.
    """
    try:
        entries = client.get_log(module_id)
    except httpx.HTTPError as error:
        print(f"    warning: could not fetch the log for {test_name}: {error}")
        return

    interesting = [
        entry
        for entry in entries
        if isinstance(entry, dict) and entry.get("result") in ("FAILURE", "WARNING")
    ]
    if not interesting:
        return

    with open(path, "a", encoding="utf-8") as handle:
        handle.write(f"\n{'=' * 78}\n{test_name} ({module_id})\n{'=' * 78}\n")
        for entry in interesting:
            handle.write(
                f"[{entry.get('result')}] {entry.get('src') or 'unknown'}: "
                f"{redact(entry.get('msg') or '')}\n"
            )
            # Conditions attach whatever they were judging. It is the evidence for the
            # failure, so keep it, but a full HTTP exchange can be enormous.
            for key, value in sorted(entry.items()):
                if key in ("result", "src", "msg", "time", "testId", "_id", "testOwner"):
                    continue
                rendered = json.dumps(redact(value, key), default=str)
                if len(rendered) > 2000:
                    rendered = rendered[:2000] + " ...truncated"
                handle.write(f"    {key}: {rendered}\n")

    print(f"    wrote {len(interesting)} log entr(ies) for {test_name} to {path}")


def asserting_modules(outcomes):
    """Return the modules that actually exercised the server.

    SKIPPED counts as passing, both in the summary table and to the certification oracle, so
    a run in which every module declined to run looks clean while having asserted nothing.
    Both verdict paths use this to require at least one module that really ran.
    """
    return [
        name
        for name, (result, _) in outcomes.items()
        if result in PASSING_RESULTS and result != "SKIPPED"
    ]


def summarise(outcomes):
    """Print a table of module outcomes and return the number of failures."""
    print("\n" + "=" * 78)
    print("OIDC CONFORMANCE RESULTS")
    print("=" * 78)

    failures = []
    for test_name, (result, status) in outcomes.items():
        marker = "PASS" if result in PASSING_RESULTS else "FAIL"
        if marker == "FAIL":
            failures.append(test_name)
        print(f"[{marker}] {test_name:<55} {result} ({status})")

    print("=" * 78)
    print(f"Total: {len(outcomes)}   Passed: {len(outcomes) - len(failures)}   Failed: {len(failures)}")
    print("=" * 78)
    return failures


def write_summary(outcomes, failures, plan_id):
    """Append a Markdown table to the GitHub Actions job summary, when running in CI."""
    summary_path = os.environ.get("GITHUB_STEP_SUMMARY")
    if not summary_path:
        return

    lines = [
        "## OIDC Conformance Results",
        "",
        f"Plan: `{plan_id}`",
        "",
        "| Result | Module | Detail |",
        "| --- | --- | --- |",
    ]
    for test_name, (result, status) in outcomes.items():
        icon = "✅" if result in PASSING_RESULTS else "❌"
        lines.append(f"| {icon} | `{test_name}` | {result} ({status}) |")

    lines += ["", f"**{len(outcomes) - len(failures)} passed, {len(failures)} failed**", ""]

    with open(summary_path, "a", encoding="utf-8") as handle:
        handle.write("\n".join(lines) + "\n")


def main():
    parser = argparse.ArgumentParser(description="Run an OIDC conformance plan against ThunderID.")
    parser.add_argument("--suite-url", default=SUITE_URL)
    parser.add_argument("--base-url", required=True, help="ThunderID base URL, e.g. https://localhost:8090")
    parser.add_argument("--config", required=True, help="Path to plan-config.json")
    parser.add_argument(
        "--profile",
        help=(
            "Named profile from plan-config.json (see its 'profiles' block). Selects the plan "
            "and whether the run drives a browser. Defaults to that file's defaultProfile."
        ),
    )
    parser.add_argument(
        "--plan",
        help=(
            "Run a plan by name instead of a profile, for a plan the config does not list. "
            "Overrides --profile."
        ),
    )
    parser.add_argument("--username", default="conformance-user")
    parser.add_argument("--password", default="Conformance@123")
    parser.add_argument(
        "--client-registration",
        default="static_client",
        help=(
            "Conformance suite client_registration variant. Defaults to static_client: the "
            "plan then uses clients provisioned up front (see --static-clients) instead of "
            "registering and deleting its own during the run."
        ),
    )
    parser.add_argument(
        "--static-clients",
        help=(
            "Path to the credentials file written by register_static_clients.py. "
            "Required by the static_client variant."
        ),
    )
    parser.add_argument(
        "--skip-certification-check",
        action="store_true",
        help=(
            "Skip the final /api/plan/{id}/certificationpackage verdict. That call publishes "
            "the plan and marks it immutable, so skip it when re-running against one plan."
        ),
    )
    parser.add_argument(
        "--response-type",
        default="code",
        help="Conformance suite response_type variant, used by the dynamic profile.",
    )
    parser.add_argument("--results-file", default="conformance-results.json")
    parser.add_argument(
        "--failure-log",
        default="conformance-failures.log",
        help=(
            "Where to append the suite's own log entries for modules that fail. "
            "Pass an empty value to skip fetching them."
        ),
    )
    parser.add_argument(
        "--no-browser",
        action="store_true",
        help="Do not drive front-channel URLs. Login-dependent modules will time out.",
    )
    parser.add_argument(
        "--headed",
        action="store_true",
        help="Run the browser with a visible window, for debugging a failing login locally.",
    )
    args = parser.parse_args()

    client = ConformanceClient(args.suite_url)

    profiles, default_profile = load_profiles(args.config)
    profile_name = args.profile or default_profile
    if args.plan:
        # An explicit plan wins, and carries no profile defaults with it.
        plan_name = args.plan
        profile = {}
    else:
        if profile_name not in profiles:
            sys.exit(
                f"ERROR: unknown profile '{profile_name}'. "
                f"Available: {', '.join(sorted(profiles))}"
            )
        profile = profiles[profile_name]
        plan_name = profile["plan"]
        print(f">>> Profile '{profile_name}': {profile.get('description', plan_name)}")

    # A plan with no front channel has nothing for the browser to drive.
    if not args.no_browser and profile.get("browser") is False:
        args.no_browser = True
        print(">>> This profile has no front-channel steps; not starting a browser")

    if args.client_registration == "static_client" and not args.static_clients:
        sys.exit(
            "ERROR: --static-clients is required with client_registration=static_client.\n"
            "       Run register_static_clients.py first to provision them."
        )

    config = load_config(args.config, args.base_url, args.static_clients)

    # ThunderID advertises `code` only and supports the `query` response mode, which is exactly
    # what the basic certification profile exercises. Which of these the plan actually accepts
    # varies, so offer only the keys it declares: passing one it pins itself is a 400.
    declared = client.get_plan_variants(plan_name)
    candidate = {
        "server_metadata": "discovery",
        "client_registration": args.client_registration,
        # The dynamic profile selects a response type instead of the metadata/registration
        # pair. ThunderID advertises `code` only, so that is the one it can be run against.
        "response_type": args.response_type,
    }
    variant = {k: v for k, v in candidate.items() if k in declared}
    if not variant:
        print(f">>> Plan '{plan_name}' declares no selectable variants; creating it without one")

    print(f">>> Creating plan '{plan_name}' with variant {variant}")
    plan = client.create_plan(plan_name, variant, config)
    plan_id = plan["id"]

    # Plan modules are objects, not names: each carries the variant the plan resolved for it.
    modules = [(module["testModule"], module.get("variant")) for module in plan["modules"]]

    print(f">>> Plan created: {args.suite_url}/plan-detail.html?plan={plan_id}")
    print(f">>> Modules to run: {len(modules)}")

    outcomes = {}
    interrupted = False
    try:
        run_all(client, modules, plan_id, args, outcomes)
    except KeyboardInterrupt:
        # Ctrl-C or a timeout wrapper. Whatever ran is still worth reporting, so fall through
        # to the summary rather than losing it.
        interrupted = True
        print("\n>>> Interrupted. Reporting the modules that completed.")

    failures = summarise(outcomes)
    write_summary(outcomes, failures, plan_id)

    incomplete = [name for name, _ in modules if name not in outcomes]
    with open(args.results_file, "w", encoding="utf-8") as handle:
        json.dump(
            {
                "planId": plan_id,
                "planUrl": f"{args.suite_url}/plan-detail.html?plan={plan_id}",
                "modules": {name: {"result": r, "status": s} for name, (r, s) in outcomes.items()},
                "failed": failures,
                "notRun": incomplete,
                "interrupted": interrupted,
            },
            handle,
            indent=2,
        )

    if incomplete:
        print(f"\n{len(incomplete)} module(s) did not run: {', '.join(incomplete)}")
    if failures:
        print(f"\nFailed modules: {', '.join(failures)}")

    if args.skip_certification_check:
        # An incomplete run is not a pass, however well the modules that did run behaved,
        # and neither is a run in which every module was skipped.
        if failures or incomplete:
            sys.exit(1)
        if not asserting_modules(outcomes):
            print("\nFAIL: every module was skipped, so the run asserted nothing.")
            sys.exit(1)
        print("\nAll conformance modules passed.")
        return

    # The suite's own verdict is authoritative. It can legitimately differ from the table
    # above: the suite excuses modules named in its
    # certification-package-failed-tests-exception-list (oidcc-server-rotate-keys by
    # default), so a run can be conformant with a module still showing as failed locally.
    print("\n" + "=" * 78)
    print("CERTIFICATION VERDICT (conformance suite)")
    print("=" * 78)

    try:
        ok, failed_tests = client.check_certification_package(plan_id)
    except httpx.HTTPError as error:
        print(f"Could not obtain a verdict: {error}")
        sys.exit(1)

    if ok:
        if not asserting_modules(outcomes):
            print("FAIL: every module was skipped, so the run asserted nothing.")
            sys.exit(1)
        print("PASS: the suite considers this plan certification-clean.")
        return

    print("FAIL: the suite rejected the plan.")
    for module_name, detail in sorted(failed_tests.items()):
        print(f"  {module_name}: {detail}")
    sys.exit(1)


def run_all(client, modules, plan_id, args, outcomes):
    """Run every module in the plan, recording each outcome in ``outcomes`` as it completes.

    The caller owns the dict so that an interrupted run still reports everything that
    finished before the interruption.
    """
    def record(test_name, module_variant, driver=None):
        result, status = run_module(client, test_name, plan_id, module_variant, driver)
        outcomes[test_name] = (result, status)
        # Capture the evidence now: the suite is torn down at the end of the job, so its
        # own log for this module is gone by the time anyone reads the results.
        if result not in PASSING_RESULTS and client.last_module_id and args.failure_log:
            dump_failure_log(client, test_name, client.last_module_id, args.failure_log)

    if args.no_browser:
        for test_name, module_variant in modules:
            record(test_name, module_variant)
        return

    # One browser for the whole plan; each visit gets a fresh context so no session
    # leaks between modules. Modules that assume a logged-out user would otherwise fail.
    with BrowserDriver(
        client,
        args.suite_url,
        args.username,
        args.password,
        headless=not args.headed,
    ) as driver:
        for test_name, module_variant in modules:
            record(test_name, module_variant, driver)


if __name__ == "__main__":
    main()
