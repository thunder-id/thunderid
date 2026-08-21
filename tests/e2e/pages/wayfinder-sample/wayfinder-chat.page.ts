// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Wayfinder AI Agent Chat Widget Page Object
 *
 * Drives the concierge chat widget embedded in the Wayfinder sample app's frontend (see
 * samples/apps/wayfinder-sample/frontend/src/App.jsx, ChatWidgetCore). The widget has no test
 * ids - every locator here is role/aria/class based, matched against that component's actual
 * markup.
 *
 * Sending the first chat message of a browser session has a side effect worth calling out: with
 * no chat access token cached yet, chatTokenService.getChatAccessToken({interactive: true}) opens
 * its own OAuth popup (scope=agent:access) to obtain one before the message ever reaches the
 * agent - this happens for every signed-in user, including one that lacks agent:access, since the
 * scope reduction happens silently server-side. sendFirstMessage() drives that popup; plain
 * sendMessage() assumes a token is already cached.
 *
 * A second, unrelated popup exists for the "act on behalf of user" (OBO) booking flow: when the
 * agent asks for consent, an in-chat bubble (ConsentRequestBubble) offers an "Authorize" button
 * that opens its own popup for the consent screen - see authorizeConsentAndGetPopup(). That popup
 * re-authenticates (if there's no active gate SSO session) and then always shows the gate's own
 * OAuth permission-approval screen (ConsentAdapter.tsx) - a per-scope toggle list defaulting to
 * off, plus Allow/Deny buttons - which completeConsentAuthorization() must grant before the popup
 * closes itself.
 */

import { Page, Locator, expect } from "@playwright/test";
import { BasePage } from "../base.page";
import { GateLoginPage } from "../gate-login.page";
import { Timeouts } from "../../constants/timeouts";

export class WayfinderChatPage extends BasePage {
  readonly launcherButton: Locator;
  readonly panel: Locator;
  readonly composerInput: Locator;
  readonly sendButton: Locator;
  // Final assistant replies only - excludes the typing indicator and consent-request bubbles,
  // which also carry the chat-message--assistant class (see App.jsx's message rendering).
  readonly assistantMessages: Locator;
  readonly typingIndicator: Locator;
  readonly consentBubble: Locator;
  readonly consentAuthorizeButton: Locator;
  readonly consentNotNowButton: Locator;

  constructor(page: Page) {
    super(page);
    // Structural, not role/name based: while the panel is open, the in-panel header close button
    // (App.jsx's chat-icon-button) carries the same "Close AI chat" aria-label as this button, so
    // a name-based query is ambiguous whenever the panel is open (i.e. exactly when reopen() needs
    // to click it).
    this.launcherButton = page.locator("button.chat-launcher");
    this.panel = page.getByRole("region", { name: "AI travel assistant" });
    // Structural, not placeholder based: the placeholder text itself changes to "Chat unavailable"
    // in the access-denied state (see expectChatUnavailable()), so a placeholder query wouldn't
    // resolve to anything once that state is reached.
    this.composerInput = page.locator(".chat-composer input");
    this.sendButton = page.getByRole("button", { name: "Send message" });
    this.assistantMessages = page.locator(
      ".chat-message.chat-message--assistant:not(.chat-message--consent):not(.chat-message--typing)"
    );
    this.typingIndicator = page.locator(".chat-message--typing");
    this.consentBubble = page.locator(".chat-message--consent");
    this.consentAuthorizeButton = this.consentBubble.getByRole("button", { name: "Authorize" });
    this.consentNotNowButton = this.consentBubble.getByRole("button", { name: "Not now" });
  }

  /** Open the chat widget if it isn't already. */
  async open() {
    if (await this.panel.isVisible().catch(() => false)) return;
    await this.launcherButton.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await this.launcherButton.click();
    await expect(this.panel).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  /**
   * Close and reopen the chat widget. The chat-access 403 check only runs when the panel opens
   * with a token already cached (see ChatWidgetCore's checkAccess effect), so re-triggering `isOpen`
   * is how a rejected user's composer actually flips to the disabled "Chat unavailable" state.
   */
  async reopen() {
    await this.launcherButton.click();
    await expect(this.panel).toBeHidden({ timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.launcherButton.click();
    await expect(this.panel).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  /**
   * Send the first chat message of the session. With no chat access token cached, this triggers
   * chatTokenService's own OAuth popup for the `agent:access` scope - sign in inside it if a login
   * form appears (an active gate SSO session skips straight through), then grant the gate's own
   * OAuth permission-approval screen if `agent:access` hasn't been consented to before (see
   * completeConsentAuthorization()'s doc comment for why that screen isn't optional) before letting
   * the popup close on its own.
   * @param text - Message to send
   * @param username - Credentials for the popup's login form, if one appears
   * @param password - Credentials for the popup's login form, if one appears
   */
  async sendFirstMessage(text: string, username: string, password: string) {
    await this.composerInput.fill(text);
    const [popup] = await Promise.all([
      this.page.waitForEvent("popup", { timeout: Timeouts.DEFAULT_ACTION }),
      this.sendButton.click(),
    ]);

    // chatTokenService opens the popup with window.open("", ...) (about:blank) and only sets its
    // location.href once the PKCE challenge is computed asynchronously - so waitForLoadState() here
    // resolves against about:blank, not the gate. Wait for the real navigation first, or the
    // login/consent race right after would start timing against about:blank instead of the gate.
    await popup.waitForURL(url => url.protocol !== "about:", { timeout: Timeouts.DEFAULT_ACTION }).catch(() => {});
    await popup.waitForLoadState();
    const outcome = await this.raceLoginConsentClose(popup);
    if (outcome === "login") {
      await new GateLoginPage(popup).login(username, password);
    }
    await this.grantAllPermissions(popup);
    await this.waitForPopupClose(popup);
  }

  /** Send a chat message once a chat access token is already cached (no popup expected). */
  async sendMessage(text: string) {
    await this.composerInput.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await this.composerInput.fill(text);
    await this.sendButton.click();
  }

  /**
   * Wait for a new assistant reply to arrive: the typing indicator (if it appeared at all)
   * settles, and the count of final assistant bubbles grows past what it was before the message
   * that triggered this reply was sent.
   * @param previousCount - Number of assistant bubbles present before sending
   */
  async waitForAssistantReply(previousCount: number) {
    await expect(this.typingIndicator).toHaveCount(0, { timeout: Timeouts.LLM_RESPONSE });
    await expect(this.assistantMessages).not.toHaveCount(previousCount, { timeout: Timeouts.LLM_RESPONSE });
  }

  async assistantMessageCount(): Promise<number> {
    return this.assistantMessages.count();
  }

  async lastAssistantMessage(): Promise<string> {
    return (await this.assistantMessages.last().textContent()) ?? "";
  }

  /** Assert the composer reflects a 403'd /chat/access check (see ChatWidgetCore's accessDenied state). */
  async expectChatUnavailable() {
    await expect(this.composerInput).toHaveAttribute("placeholder", "Chat unavailable");
    await expect(this.composerInput).toBeDisabled();
  }

  async waitForConsentRequest() {
    await expect(this.consentBubble).toBeVisible({ timeout: Timeouts.LLM_RESPONSE });
  }

  /** Click "Authorize" on the in-chat consent bubble and return the OAuth consent popup it opens. */
  async authorizeConsentAndGetPopup(): Promise<Page> {
    const [popup] = await Promise.all([
      this.page.waitForEvent("popup", { timeout: Timeouts.DEFAULT_ACTION }),
      this.consentAuthorizeButton.click(),
    ]);
    await popup.waitForLoadState();
    return popup;
  }

  /**
   * Complete the OAuth consent popup opened by authorizeConsentAndGetPopup(): sign in if a login
   * form appears (an active gate SSO session skips straight through), then grant every permission
   * on the gate's own OAuth permission-approval screen that follows - it always appears, toggles
   * default off, and the popup won't close until "Allow" is clicked.
   * @param popup - Popup returned by authorizeConsentAndGetPopup()
   * @param username - Credentials for the popup's login form, if one appears
   * @param password - Credentials for the popup's login form, if one appears
   */
  async completeConsentAuthorization(popup: Page, username: string, password: string) {
    const outcome = await this.raceLoginConsentClose(popup);
    if (outcome === "login") {
      await new GateLoginPage(popup).login(username, password);
    }

    await this.grantAllPermissions(popup);

    await this.waitForPopupClose(popup);
  }

  /**
   * Toggle on every permission switch on the gate's OAuth permission-approval screen (see
   * ConsentAdapter.tsx - each requested scope renders as an off-by-default Switch) and click
   * "Allow". A no-op if the screen never appears (e.g. scopes already granted in a prior test run).
   * @param popup - Popup currently showing (or about to show) the permission-approval screen
   */
  private async grantAllPermissions(popup: Page) {
    const allowButton = popup.getByRole("button", { name: "Allow" });
    const consentScreenVisible = await this.waitVisibleOrPopupClosed(allowButton, popup);
    if (!consentScreenVisible) return;

    const permissionSwitches = popup.getByRole("switch");
    const count = await permissionSwitches.count();
    for (let i = 0; i < count; i++) {
      const permissionSwitch = permissionSwitches.nth(i);
      if (!(await permissionSwitch.isChecked())) {
        await permissionSwitch.click();
      }
    }
    await allowButton.click();
  }

  /**
   * Wait for the OAuth popup to close itself. With an active gate SSO session, the popup redirects
   * and self-closes right after grantAllPermissions() returns - often before this call is even
   * reached - so waitForEvent("close") would miss the event and stall for the full REDIRECT
   * timeout. Only register the listener when the popup is still open.
   */
  private async waitForPopupClose(popup: Page) {
    if (popup.isClosed()) return;
    await popup.waitForEvent("close", { timeout: Timeouts.REDIRECT }).catch(() => {});
  }

  /**
   * Race the login form, the consent screen's "Allow" button, and popup closure against each
   * other and report whichever appears first. An active gate SSO session can skip the login form
   * entirely and land straight on the consent screen, or skip both and self-close the popup if
   * consent was already granted - a fixed probe timeout for the login form alone can't tell a
   * "this step is skipped" popup from a "the login form is just slow to render" one, so all three
   * outcomes are raced against the same timeout instead of probing the login form in isolation.
   */
  private async raceLoginConsentClose(
    popup: Page,
    timeout: number = Timeouts.DEFAULT_ACTION
  ): Promise<"login" | "consent" | "closed"> {
    if (popup.isClosed()) return "closed";
    const usernameInput = popup.locator('input[name="username"], input[placeholder*="username" i]').first();
    const allowButton = popup.getByRole("button", { name: "Allow" });
    try {
      return await Promise.any([
        usernameInput.waitFor({ state: "visible", timeout }).then(() => "login" as const),
        allowButton.waitFor({ state: "visible", timeout }).then(() => "consent" as const),
        popup.waitForEvent("close", { timeout }).then(() => "closed" as const),
      ]);
    } catch {
      return "closed";
    }
  }

  /**
   * Wait for a step (login form, consent screen) to appear, but give up as soon as the popup
   * closes instead. An active gate SSO session can skip straight past a step and let the popup
   * self-close before it ever appears - without this race, the locator would poll for the full
   * timeout since "closed" isn't a state waitFor() treats as failure on its own.
   *
   * Only remaining caller is grantAllPermissions(), reached either right after a login form
   * submission (a real redirect may still be in flight) or right after raceLoginConsentClose()
   * already found the consent screen (resolves immediately) - the default timeout covers both.
   */
  private async waitVisibleOrPopupClosed(
    locator: Locator,
    popup: Page,
    timeout: number = Timeouts.DEFAULT_ACTION
  ): Promise<boolean> {
    if (popup.isClosed()) return false;
    return Promise.race([
      locator.waitFor({ state: "visible", timeout }).then(
        () => true,
        () => false
      ),
      popup.waitForEvent("close", { timeout }).then(
        () => false,
        () => false
      ),
    ]);
  }

  /** True once the consent bubble reports the popup flow completed ("Authorized — retrying…"). */
  async waitForConsentAuthorized() {
    await expect(this.consentBubble.getByText(/authorized/i)).toBeVisible({ timeout: Timeouts.LLM_RESPONSE });
  }
}
