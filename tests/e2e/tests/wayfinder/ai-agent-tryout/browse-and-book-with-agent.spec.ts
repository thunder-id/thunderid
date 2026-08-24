/* eslint-disable playwright/require-top-level-describe */
// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Wayfinder AI Agent Tryout - Browse and Book with Agent E2E Tests
 *
 * Mirrors docs/content/use-cases/ai-agents/try-it-out/act-on-its-own.mdx and
 * act-on-behalf-of-user.mdx as one continuous chat session (the agent keeps conversation state
 * keyed by session_id): the concierge first answers browsing questions using its own M2M token
 * against the booking API's MCP tools (no user consent required), then, in the same session,
 * booking a flight mutates the user's record, so the agent pauses for consent: an in-chat bubble
 * names the requested `booking:*` scope, and a popup asks the user to re-authenticate before the
 * booking completes on a delegated (OBO) token. Both walkthroughs share the same sign-in and
 * first chat message, so they run as steps of a single test rather than as separate ones.
 *
 * This spec must run after wayfinder-sample-setup.spec.ts has imported the Wayfinder bundle - see
 * ../../b2c-tryout/authentication.spec.ts's header - and needs the Wayfinder AI agent service
 * actually running, which run-e2e.sh only starts when a real LLM key is configured (see
 * tests/e2e/.env.example) - skipped entirely otherwise.
 *
 * LLM output is not deterministic: the tool-choice steps below (asking for flights, then asking to
 * book) rely on the agent recognising a mutating request and returning `need_user_consent`, per
 * the doc walkthrough; assertions otherwise only check that a reply arrived and loosely matches
 * the topic asked about, not any exact wording.
 *
 * Required environment variables:
 * - WAYFINDER_APP_URL: URL of the running Wayfinder sample app (e.g. http://localhost:5173)
 * - LLM_API_KEY: API key for the configured LLM provider (see tests/e2e/.env.example)
 */

import { test, expect } from "@playwright/test";
import { WayfinderAppPage, WayfinderChatPage } from "../../../pages/wayfinder-sample";
import { TestTags } from "../../../constants/test-tags";

const wayfinderUrl = process.env.WAYFINDER_APP_URL;

// Seed user shipped with the Wayfinder bundle - see
// samples/apps/wayfinder-sample/thunderid-config/redirect/thunderid.env
const JOHN_USERNAME = "john.doe";
const JOHN_PASSWORD = "john.doe";

// The AI agent needs a real LLM; run-e2e.sh only starts it when LLM_API_KEY is set, so skip
// entirely otherwise (see tests/e2e/.env.example).
const describeOrSkip = wayfinderUrl && process.env.LLM_API_KEY ? test.describe : test.describe.skip;

describeOrSkip("Wayfinder AI Agent Tryout - Browse and Book with Agent", { tag: [TestTags.WAYFINDER] }, () => {
  test("TC001: Browse flights with the concierge's own token, then book one with delegated consent", async ({
    page,
  }) => {
    const wayfinderPage = new WayfinderAppPage(page);
    const chatPage = new WayfinderChatPage(page);
    let beforeConsent = 0;

    await test.step("Sign in as john.doe", async () => {
      await wayfinderPage.goto(wayfinderUrl!);
      await wayfinderPage.verifyUnAuthenticatedHomePageLoaded();
      await wayfinderPage.clickSignInButton();
      await wayfinderPage.verifyLoginPageLoaded();
      await wayfinderPage.login(JOHN_USERNAME, JOHN_PASSWORD);
      await wayfinderPage.verifyLoggedIn();
    });

    await test.step("Ask for flights from Colombo to Singapore", async () => {
      await chatPage.open();
      const before = await chatPage.assistantMessageCount();
      // First chat message of the session: no chat-access token is cached yet, so sending it
      // drives chatTokenService's own OAuth popup - see sendFirstMessage()'s doc comment.
      await chatPage.sendFirstMessage(
        "What flights are there from Colombo to Singapore?",
        JOHN_USERNAME,
        JOHN_PASSWORD
      );
      await chatPage.waitForAssistantReply(before);
      await expect(chatPage.assistantMessages.last()).toContainText(/colombo|singapore|flight/i);
    });

    // Regression check for the bug this suite guards against: browsing must never trigger a
    // second, mid-conversation auth popup - only the "act on behalf of user" (OBO) booking flow
    // below may. The chat-access token acquired above is cached for the rest of the session, so
    // this follow-up message should need no popup of its own.
    let popupOpened = false;
    page.on("popup", () => {
      popupOpened = true;
    });

    await test.step("Ask for flight deal recommendations", async () => {
      const before = await chatPage.assistantMessageCount();
      await chatPage.sendMessage("Suggest a few flight deals.");
      await chatPage.waitForAssistantReply(before);
      const reply = await chatPage.lastAssistantMessage();
      expect(reply.trim().length).toBeGreaterThan(0);
    });

    expect(popupOpened).toBe(false);

    await test.step("Ask to book a flight and receive a consent request", async () => {
      beforeConsent = await chatPage.assistantMessageCount();
      await chatPage.sendMessage("Book flight 2");
      await chatPage.waitForConsentRequest();
      // The agent's response to a mutating request isn't plain text - it's the in-chat consent
      // bubble (ConsentRequestBubble in App.jsx), pausing for an explicit "Authorize" click before
      // the OAuth popup for the delegated token ever opens.
      await expect(chatPage.consentBubble).toContainText(
        "This action needs your permission. Sign in to authorize the assistant to act on your behalf."
      );
      await expect(chatPage.consentBubble).toContainText(/Scope requested:.*booking/);
      await expect(chatPage.consentAuthorizeButton).toBeVisible();
      await expect(chatPage.consentNotNowButton).toBeVisible();
    });

    await test.step("Authorize the booking by signing in and granting permissions in the popup", async () => {
      // The popup re-authenticates (skipped if there's an active gate SSO session) and then always
      // shows the gate's own OAuth permission-approval screen, which completeConsentAuthorization()
      // grants before the popup closes itself.
      const popup = await chatPage.authorizeConsentAndGetPopup();
      await chatPage.completeConsentAuthorization(popup, JOHN_USERNAME, JOHN_PASSWORD);
      await chatPage.waitForConsentAuthorized();
    });

    await test.step("Verify the booking is confirmed in chat", async () => {
      // The agent auto-retries the pending "Book flight 2" message once the OBO token lands (see
      // App.jsx's postMessage handler), appending a new assistant reply.
      await chatPage.waitForAssistantReply(beforeConsent);
      const reply = await chatPage.lastAssistantMessage();
      // create_booking's result is always rendered from the "Booking WF-XXXXXXXX (confirmed)"
      // template (see wayfinder-sample/ai-agent/agent.ts's system prompt and
      // backend/src/mcp.js's generateBookingReference) - the route/airline/price that follow vary
      // with whichever flight deal "flight 2" resolved to, so only the reference format and
      // confirmation wording are pinned here, not those non-deterministic specifics.
      expect(reply).toMatch(/booking\s+wf-[0-9a-f]{8}/i);
      expect(reply).toMatch(/confirmed/i);
    });
  });
});
