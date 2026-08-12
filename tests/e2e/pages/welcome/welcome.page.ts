// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Welcome / Wayfinder Sample Setup Page Object Model
 *
 * Encapsulates the welcome screen (shown on first login) and the embedded "Setup Wayfinder
 * Sample" card that imports the Wayfinder declarative config bundle into ThunderID.
 *
 * There are no data-testid attributes in the production DOM for this feature, so every locator
 * below is role- and text-based (verbatim en-US copy). Welcome state (dismissed / already
 * configured / expanded) lives in sessionStorage, keyed as `{productname-lowercased}:<suffix>` -
 * the suffix-based helpers here avoid hardcoding the product name.
 *
 * @example
 * const welcomePage = new WelcomePage(page, baseUrl);
 * await welcomePage.simulateFirstStart();
 * await welcomePage.page.goto(`${baseUrl}${ConsoleRoutes.dashboard}`);
 * await welcomePage.verifyOnWelcomeScreen();
 */

import { Page, Locator, Download, Response, expect } from "@playwright/test";
import { ConsoleRoutes } from "../../configs/routes/console-routes";
import { BasePage } from "../base.page";
import { Timeouts } from "../../constants/timeouts";

/** Suffix of the sessionStorage key that tracks whether the welcome screen has been dismissed. */
const WELCOME_DISMISSED_SUFFIX = ":welcome:dismissed";

/** Suffix of the sessionStorage key that tracks whether the Wayfinder bundle has been imported. */
const WAYFINDER_CONFIGURED_SUFFIX = ":wayfinder-config-imported";

/** Matches the release asset name WayfinderSampleDownload resolves via useWayfinderReleases. */
const WAYFINDER_ASSET_NAME_PATTERN = /^sample-app-wayfinder-[0-9A-Za-z.+-]+\.zip$/i;

/** Matches the download link's href: a full URL ending in the asset name above, not the bare name. */
const WAYFINDER_ASSET_URL_PATTERN = /\/sample-app-wayfinder-[0-9A-Za-z.+-]+\.zip$/i;

export class WelcomePage extends BasePage {
  readonly baseUrl: string;

  // Welcome screen (hero + navigation)
  readonly heroHeading: Locator;
  readonly tryoutSecuringApplicationRow: Locator;
  readonly tryoutAiAgentsRow: Locator;
  readonly tryoutMcpRow: Locator;

  // Dashboard header (reopening welcome via the user menu)
  readonly userMenuTrigger: Locator;
  readonly welcomeMenuItem: Locator;

  // Wayfinder setup card
  readonly setupCardHeader: Locator;
  readonly getSampleStepTitle: Locator;
  readonly downloadLink: Locator;
  readonly configureStepTitle: Locator;
  readonly runStepTitle: Locator;

  // Config import sub-step
  readonly importButton: Locator;
  readonly importSuccessLabel: Locator;
  readonly importResourcesImportedLabel: Locator;
  readonly importAlreadyDoneLabel: Locator;
  readonly importLastImportedLabel: Locator;
  readonly reconfigureButton: Locator;
  readonly importErrorLabel: Locator;

  constructor(page: Page, baseUrl: string) {
    super(page);
    this.baseUrl = baseUrl;

    // Welcome screen
    this.heroHeading = page.getByRole("heading", { name: /welcome to/i });
    this.tryoutSecuringApplicationRow = page.getByRole("button", { name: /secured web application/i });
    this.tryoutAiAgentsRow = page.getByRole("button", { name: /secured ai agent/i });
    this.tryoutMcpRow = page.getByRole("button", { name: /secured mcp server/i });

    // Dashboard header. The header renders as an HTML <header> element (MUI AppBar's default
    // root), and the user-menu trigger is the only element inside it with aria-haspopup="true".
    this.userMenuTrigger = page.locator('header [aria-haspopup="true"]');
    this.welcomeMenuItem = page.getByRole("menuitem", { name: /^welcome$/i });

    // Wayfinder setup card
    this.setupCardHeader = page.getByRole("button", { name: /setup wayfinder sample/i });
    this.getSampleStepTitle = page.getByText(/get the wayfinder sample/i);
    this.downloadLink = page.getByRole("link", { name: /^download$/i });
    this.configureStepTitle = page.getByText(/^configure wayfinder sample in/i);
    this.runStepTitle = page.getByText(/run the sample/i);

    // Config import sub-step
    this.importButton = page.getByRole("button", { name: /^configure in/i });
    this.importSuccessLabel = page.getByText(/wayfinder sample configured in .+ successfully/i);
    this.importResourcesImportedLabel = page.getByText(/resources? imported/i);
    this.importAlreadyDoneLabel = page.getByText(/wayfinder sample already configured in/i);
    this.importLastImportedLabel = page.getByText(/last configured on/i);
    this.reconfigureButton = page.getByRole("button", { name: /^reconfigure$/i });
    this.importErrorLabel = page.getByText(/import failed|failed resource/i);
  }

  /** Navigate to the welcome screen. Callers follow this with their own explicit wait. */
  async goto(): Promise<void> {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.welcome}`, {
      timeout: Timeouts.PAGE_LOAD,
    });
  }

  /**
   * Navigate to the "Secured Web Application" tryout page, which embeds the Wayfinder setup
   * card. Callers follow this with their own explicit wait (expandSetupCardIfCollapsed(), etc.).
   */
  async gotoTryoutApp(): Promise<void> {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.welcomeTryoutApp}`, {
      timeout: Timeouts.PAGE_LOAD,
    });
  }

  /**
   * Assert the browser is currently showing the welcome screen. Polls the URL rather than
   * checking it once: WelcomeRedirect only navigates here from a client-side useEffect, which
   * page.goto()'s load event does not wait for.
   */
  async verifyOnWelcomeScreen(): Promise<void> {
    await expect(this.page).toHaveURL(new RegExp(ConsoleRoutes.welcome), { timeout: Timeouts.ELEMENT_VISIBILITY });
    await expect(this.heroHeading).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  /** Open the user menu and click "Welcome" to reopen the welcome screen. */
  async reopenFromUserMenu(): Promise<void> {
    await this.userMenuTrigger.click();
    await this.welcomeMenuItem.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.welcomeMenuItem.click();
  }

  /**
   * Remove any sessionStorage key tracking welcome-dismissed state, for every page in this
   * page's browser context, before each future navigation. Combined with a subsequent
   * `page.goto(...)`, this simulates the very first application start: `WelcomeRedirect` reads an
   * absent flag and redirects to `/welcome`. Registered via `context.addInitScript`, which runs
   * after the auth fixture's own init script (registered earlier), so this one wins and clears
   * the flag the auth script just set.
   */
  async simulateFirstStart(): Promise<void> {
    await this.page.context().addInitScript((suffix: string) => {
      for (let i = sessionStorage.length - 1; i >= 0; i--) {
        const key = sessionStorage.key(i);
        if (key && key.endsWith(suffix)) sessionStorage.removeItem(key);
      }
    }, WELCOME_DISMISSED_SUFFIX);
  }

  /** Whether the welcome-dismissed flag is currently set to `'true'` in sessionStorage. */
  async isWelcomeDismissed(): Promise<boolean> {
    return this.page.evaluate((suffix: string) => {
      for (let i = 0; i < sessionStorage.length; i++) {
        const key = sessionStorage.key(i);
        if (key && key.endsWith(suffix)) return sessionStorage.getItem(key) === "true";
      }
      return false;
    }, WELCOME_DISMISSED_SUFFIX);
  }

  /** Whether the "already imported" sessionStorage flag is currently set. */
  async isWayfinderConfiguredFlagSet(): Promise<boolean> {
    return this.page.evaluate((suffix: string) => {
      for (let i = 0; i < sessionStorage.length; i++) {
        const key = sessionStorage.key(i);
        if (key && key.endsWith(suffix)) return !!sessionStorage.getItem(key);
      }
      return false;
    }, WAYFINDER_CONFIGURED_SUFFIX);
  }

  /** Expand the "Setup Wayfinder Sample" card if it is currently collapsed. */
  async expandSetupCardIfCollapsed(): Promise<void> {
    await this.setupCardHeader.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    const expanded = await this.setupCardHeader.getAttribute("aria-expanded");
    if (expanded !== "true") {
      await this.setupCardHeader.click();
    }
    await expect(this.setupCardHeader).toHaveAttribute("aria-expanded", "true");
  }

  /**
   * Click the wayfinder sample download link and assert a download starts. The link resolves to
   * a release asset fetched at runtime (see useWayfinderReleases), so this needs live network
   * access to whatever releasesUrl the app is configured with.
   */
  async triggerDownload(): Promise<Download> {
    await this.downloadLink.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await expect(this.downloadLink).toHaveAttribute("href", WAYFINDER_ASSET_URL_PATTERN);

    const [download] = await Promise.all([
      this.page.context().waitForEvent("download", { timeout: Timeouts.PAGE_LOAD }),
      this.downloadLink.click(),
    ]);
    expect(download.suggestedFilename()).toMatch(WAYFINDER_ASSET_NAME_PATTERN);
    return download;
  }

  /**
   * Click the "Configure in ThunderID" import button and wait for the `/import` API call to
   * complete. Returns the response so the caller can assert on its status.
   */
  async runImportAndWait(): Promise<Response> {
    await this.importButton.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    const [response] = await Promise.all([
      this.page.waitForResponse(resp => resp.url().includes("/import") && resp.request().method() === "POST", {
        timeout: Timeouts.PAGE_LOAD,
      }),
      this.importButton.click(),
    ]);
    return response;
  }
}
