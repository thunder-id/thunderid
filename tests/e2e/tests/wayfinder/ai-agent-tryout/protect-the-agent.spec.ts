/* eslint-disable playwright/require-top-level-describe */
// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Wayfinder AI Agent Tryout - Protect the Agent E2E Tests
 *
 * Mirrors docs/content/use-cases/ai-agents/try-it-out/protect-the-agent.mdx: `john.doe` holds the
 * `agent:access` permission (via the `Chat User` role) and can use the Wayfinder Concierge chat;
 * `jane.smith` does not and is rejected with a 403.
 *
 * This spec must run after wayfinder-sample-setup.spec.ts has imported the Wayfinder bundle (the
 * seed users only exist afterward - enforced structurally by the wayfinder-setup -> wayfinder-tryout
 * project dependency in playwright.config.ts, same as ../../b2c-tryout/authentication.spec.ts) and
 * needs the Wayfinder AI agent service actually running, which run-e2e.sh only starts when a real
 * LLM key is configured (see tests/e2e/.env.example) - skipped entirely otherwise.
 *
 * Required environment variables:
 * - WAYFINDER_APP_URL: URL of the running Wayfinder sample app (e.g. http://localhost:5173)
 * - LLM_API_KEY: API key for the configured LLM provider (see tests/e2e/.env.example)
 */

import { test, expect } from "@playwright/test";
import { WayfinderAppPage, WayfinderChatPage } from "../../../pages/wayfinder-sample";
import { TestTags } from "../../../constants/test-tags";
import { Timeouts } from "../../../constants/timeouts";

const wayfinderUrl = process.env.WAYFINDER_APP_URL;

// Seed users shipped with the Wayfinder bundle - see
// samples/apps/wayfinder-sample/thunderid-config/redirect/thunderid.env
const JOHN_USERNAME = "john.doe";
const JOHN_PASSWORD = "john.doe";
const JANE_USERNAME = "jane.smith";
const JANE_PASSWORD = "jane.smith";

// The AI agent needs a real LLM; run-e2e.sh only starts it when LLM_API_KEY is set, so skip
// entirely otherwise (see tests/e2e/.env.example).
const describeOrSkip = wayfinderUrl && process.env.LLM_API_KEY ? test.describe : test.describe.skip;

describeOrSkip("Wayfinder AI Agent Tryout - Protect the Agent", { tag: [TestTags.WAYFINDER] }, () => {
  test("TC001: john.doe holds agent:access and receives a reply from the concierge", async ({ page }) => {
    const wayfinderPage = new WayfinderAppPage(page);
    const chatPage = new WayfinderChatPage(page);

    await test.step("Sign in as john.doe", async () => {
      await wayfinderPage.goto(wayfinderUrl!);
      await wayfinderPage.verifyUnAuthenticatedHomePageLoaded();
      await wayfinderPage.clickSignInButton();
      await wayfinderPage.verifyLoginPageLoaded();
      await wayfinderPage.login(JOHN_USERNAME, JOHN_PASSWORD);
      await wayfinderPage.verifyLoggedIn();
    });

    await test.step("Open the chat and send a message", async () => {
      await chatPage.open();
      const before = await chatPage.assistantMessageCount();
      // First chat message of the session: no chat-access token is cached yet, so sending it
      // drives chatTokenService's own OAuth popup - see sendFirstMessage()'s doc comment.
      await chatPage.sendFirstMessage("Hello, can you help me?", JOHN_USERNAME, JOHN_PASSWORD);
      await chatPage.waitForAssistantReply(before);
    });

    await test.step("Verify a reply arrived", async () => {
      const reply = await chatPage.lastAssistantMessage();
      expect(reply.trim().length).toBeGreaterThan(0);
    });
  });

  test("TC002: jane.smith lacks agent:access and is rejected", async ({ page }) => {
    const wayfinderPage = new WayfinderAppPage(page);
    const chatPage = new WayfinderChatPage(page);

    await test.step("Sign in as jane.smith", async () => {
      await wayfinderPage.goto(wayfinderUrl!);
      await wayfinderPage.verifyUnAuthenticatedHomePageLoaded();
      await wayfinderPage.clickSignInButton();
      await wayfinderPage.verifyLoginPageLoaded();
      await wayfinderPage.login(JANE_USERNAME, JANE_PASSWORD);
      await wayfinderPage.verifyLoggedIn();
    });

    await test.step("Open the chat and verify it is rejected", async () => {
      // No chat-access token is cached yet, so ChatWidgetCore's checkAccess effect (which only
      // ever looks at a cached token) no-ops on open - the rejection only surfaces once Jane
      // actually sends a message: that drives the same interactive OAuth popup John went through
      // (the scope reduction to "none of agent:access" happens silently server-side), and the
      // /chat request made with that token comes back with the 403 error below.
      await chatPage.open();
      await chatPage.sendFirstMessage("Hello, can you help me?", JANE_USERNAME, JANE_PASSWORD);
      await expect(chatPage.assistantMessages.last()).toContainText(
        "You do not have permission to access Wayfinder Concierge — missing required scope: agent:access",
        { timeout: Timeouts.LLM_RESPONSE }
      );

      // The token from that popup is now cached (just scoped short of agent:access), so
      // reopening the panel re-runs checkAccess against it - see reopen()'s doc comment - which
      // is what actually flips the composer into its disabled "Chat unavailable" state.
      await chatPage.reopen();
      await chatPage.expectChatUnavailable();
    });
  });
});
