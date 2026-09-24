# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

"""Complete the conformance suite's front-channel steps in a real browser.

The suite drives front-channel interaction with HtmlUnit, which does not execute ES modules.
The gate login page is a client-side React app whose fields are not in the served HTML at all:
the bundle calls /flow/execute and renders whatever the configured authentication flow returns.
HtmlUnit therefore never renders a form and every login-dependent module times out.

With no matching `browser` entry in the plan config, BrowserControl.goToUrl() leaves the URL in
the test's `urls` list for someone else to visit. This module is that someone: it polls
GET /api/runner/browser/{id}, opens each published URL in Chromium, signs in when a login form
appears, and reports the URL back via POST /api/runner/browser/{id}/visit.
"""

import time

from playwright.sync_api import TimeoutError as PlaywrightTimeoutError, sync_playwright

# How long to wait for the React bundle to render the sign-in form. The page has to load a
# module graph and call /flow/execute before any field exists, so this is slower than a
# server-rendered form would be.
FORM_TIMEOUT_MS = 30000

# How long to wait for the browser to arrive back at the suite after submitting credentials.
CALLBACK_TIMEOUT_MS = 30000

# Selectors for the gate sign-in form. The flow engine decides which fields to render, so these
# match what the default username/password flow produces.
USERNAME_SELECTOR = "input[name='username'], #username"
PASSWORD_SELECTOR = "input[name='password'], #password"
SUBMIT_SELECTOR = "button[type='submit']"


class BrowserDriver:
    """Visits front-channel URLs for a running test module."""

    def __init__(self, client, suite_url, username, password, headless=True, verbose=False):
        self.client = client
        self.suite_url = suite_url.rstrip("/")
        self.username = username
        self.password = password
        self.headless = headless
        self.verbose = verbose
        self._playwright = None
        self._browser = None
        self._contexts = {}

    def __enter__(self):
        self._playwright = sync_playwright().start()
        self._browser = self._playwright.chromium.launch(headless=self.headless)
        return self

    def __exit__(self, *_):
        for module_id in list(self._contexts):
            self.release(module_id)
        if self._browser:
            self._browser.close()
        if self._playwright:
            self._playwright.stop()

    def _context_for(self, module_id):
        """Return the browser context this module's visits share, creating it on first use.

        A module's front-channel steps are one browser session: modules that re-authorize
        (prompt, max_age, id_token_hint) depend on the SSO cookie set by their first visit
        still being present on the second, so the context has to outlive a single visit.
        Isolation stays at the module boundary — each module gets its own context, and
        release() drops it — so a module that expects a logged-out user still gets one.
        """
        context = self._contexts.get(module_id)
        if context is None:
            # The server and the suite both serve certificates that are not in the system
            # trust store, so verification is off for the same reason the suite's own HTTP
            # client turns it off. This talks only to the two hosts under test.
            context = self._browser.new_context(ignore_https_errors=True)
            self._contexts[module_id] = context
        return context

    def release(self, module_id):
        """Discard a finished module's context, and with it that module's cookies."""
        context = self._contexts.pop(module_id, None)
        if context is None:
            return
        try:
            context.close()
        except Exception as error:  # noqa: BLE001 - teardown must not fail the run
            if self.verbose:
                print(f"    warning: could not close browser context: {error}")

    def visit(self, url, module_id):
        """Open one front-channel URL, sign in if asked, and wait for the return to the suite."""
        page = self._context_for(module_id).new_page()
        try:
            page.goto(url, wait_until="domcontentloaded")
            if self._sign_in(page):
                self._wait_for_callback(page)
        finally:
            page.close()

    def _sign_in(self, page):
        """Fill and submit the sign-in form. Returns False if no form appeared."""
        try:
            page.wait_for_selector(USERNAME_SELECTOR, timeout=FORM_TIMEOUT_MS, state="visible")
        except PlaywrightTimeoutError:
            # Not every front-channel URL is a login: a module may be checking an error
            # response, or the flow may already have an authenticated session. Both are
            # legitimate, so let the module's own assertions decide.
            if self.verbose:
                print(f"    no sign-in form at {page.url}")
            return False

        page.fill(USERNAME_SELECTOR, self.username)
        page.fill(PASSWORD_SELECTOR, self.password)
        page.click(SUBMIT_SELECTOR)
        return True

    def _wait_for_callback(self, page):
        """Wait until the browser lands back on the suite, which owns the redirect URI."""
        try:
            page.wait_for_url(f"{self.suite_url}/**", timeout=CALLBACK_TIMEOUT_MS)
        except PlaywrightTimeoutError:
            # The module still has the authorization response if the redirect did arrive by
            # another route, so record where the browser stopped and let the module judge.
            print(f"    warning: did not return to the suite, stopped at {page.url}")

    def drive_pending(self, module_id, seen):
        """Visit any front-channel URLs the module has published but not yet had visited.

        ``seen`` carries across polls so a URL is driven once even though the suite keeps
        listing it until the visit is registered.
        """
        status = self.client.get_browser_status(module_id)
        if status is None:
            return

        for url in status.get("urls", []):
            if url in seen:
                continue
            seen.add(url)
            print(f"    visiting {url}")
            try:
                self.visit(url, module_id)
            except Exception as error:  # noqa: BLE001 - a failed visit fails the module, not the run
                print(f"    warning: browser visit failed: {error}")
            self.client.mark_url_visited(module_id, url)
