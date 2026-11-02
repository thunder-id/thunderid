// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * --product-version selects the release to install and where to read releases from.
 *
 * Only the rejection paths are covered here: they need no install and no network, and getting
 * them wrong is what would let a typo silently install the wrong thing. The accepting paths are
 * exercised for real by the product and release lanes, which run the whole suite against a
 * locally served distribution through this same flag.
 */

import { test, expect } from "@playwright/test";
import { CliSession, publishedTag } from "../../utils/cli-session";
import { Workspace } from "../../utils/workspace";

test.describe("thunderid --product-version", () => {
  // The published CLI predates the flag, so asserting it there would fail for the wrong reason.
  // This is conditional on which CLI is under test, not a parked test.
  // eslint-disable-next-line playwright/no-skipped-test
  test.skip(publishedTag() !== null, "the flag is newer than the published CLI");

  let workspace: Workspace;
  let session: CliSession;

  test.beforeEach(() => {
    workspace = Workspace.create();
  });

  test.afterEach(async () => {
    await session?.dispose();
    workspace.remove();
  });

  test("is documented in the usage output", async () => {
    session = new CliSession(workspace, ["--help"]);

    expect(await session.waitForExit()).toBe(0);
    expect(session.output).toContain("--product-version");
    expect(session.output, "the help should show both value shapes").toContain("https://example.com/releases.json");
  });

  test("rejects a value that is neither a version nor a URL", async () => {
    session = new CliSession(workspace, ["--product-version", "latest"]);

    await session.expect("neither a version nor a URL");
    expect(await session.waitForExit()).toBe(1);
  });

  // Releases are executable, so a cleartext remote source is refused rather than warned about.
  test("rejects a remote http source", async () => {
    session = new CliSession(workspace, ["--product-version", "http://releases.example.com/releases.json"]);

    await session.expect("plain http");
    expect(await session.waitForExit()).toBe(1);
  });

  test("refuses to upgrade while pinned to a version", async () => {
    session = new CliSession(workspace, ["upgrade", "--product-version", "1.0.1"]);

    await session.expect("cannot run with a pinned --product-version");
    expect(await session.waitForExit()).toBe(1);
  });
});
