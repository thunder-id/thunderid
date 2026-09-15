// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Social Login Setup Utilities
 *
 * Automated setup for social login E2E testing prerequisites, for every branded vendor the
 * backend supports (Google, GitHub):
 * - Connection (identity provider) pointing at that vendor's mock OAuth/OIDC server
 * - Authentication flow with a "Continue with <vendor>" step
 * - Application rewired to use that flow, with its previous flow bindings restored on cleanup
 *
 * Only the connection endpoint, resource names, executor and button branding differ per vendor;
 * everything else (application rewiring, linked user, cleanup) is identical, so the vendors share
 * one implementation and one flow-node template.
 *
 * All backend calls go through the `*Api` helpers (which themselves go through `send`/`sendOk`,
 * owning the admin bearer token and `ignoreHTTPSErrors`), so nothing here handles auth headers
 * directly. Resources are looked up by name/handle before creating - the vendors' suites run
 * sequentially (same spec file, and that file runs in a single browser project - see
 * SERVER_STATE_SPECS in playwright.config.ts), so at most one vendor's setup/teardown is ever live.
 * That is also why both vendors can share one dedicated application (constants/sample-apps.ts)
 * instead of needing one each: the real, unavoidable cross-vendor contention is the server-wide
 * identity_provider.<vendor>_base_url config and each mock's fixed port, not the application.
 */

import type { APIRequestContext } from "@playwright/test";
import { UsersApi } from "../users-api";
import { ConnectionsApi } from "../connections-api";
import { FlowsApi } from "../flows-api";
import { rewireApplicationFlows, restoreApplicationFlows } from "./application-flows";
import socialAuthFlowNodesTemplate from "./social-auth-flow-nodes.json";

/** Branded social vendors wired to a `/connections/<vendor>` API. */
export type SocialVendor = "google" | "github";

interface VendorProfile {
  /** Human-readable vendor name, used in resource names, the button label and log output. */
  displayName: string;
  /** Flow executor that drives this vendor's federated authentication. */
  executorName: string;
  /** Button icon served by the gate, relative to the console's public assets. */
  icon: string;
  /**
   * Scopes requested from the vendor. GitHub needs `user:email` because its /user response
   * omits the address for accounts that keep it private, and the backend then falls back to
   * the /user/emails endpoint.
   */
  scopes: string[];
}

const VENDOR_PROFILES: Record<SocialVendor, VendorProfile> = {
  google: {
    displayName: "Google",
    executorName: "GoogleOIDCAuthExecutor",
    icon: "assets/images/icons/google.svg",
    scopes: ["openid", "email", "profile"],
  },
  github: {
    displayName: "GitHub",
    executorName: "GithubOAuthExecutor",
    icon: "assets/images/icons/github.svg",
    scopes: ["read:user", "user:email"],
  },
};

export interface SocialLinkedUser {
  username: string;
  email: string;
  /**
   * Must match the `sub` claim the vendor's mock server issues for this identity. For Google
   * that is the ID token's `sub`; for GitHub it is the numeric account id as a string, which
   * the backend renames to `sub` (see backend/internal/authn/oauth/utils.go::ProcessSubClaim).
   */
  sub: string;
}

export interface SocialLoginSetupConfig {
  vendor: SocialVendor;
  /** clientId of the application to rewire (constants/sample-apps.ts) - distinct from `clientId`
   * below, which is the mock vendor's own OAuth client id for the connection. */
  appClientId: string;
  clientId: string;
  clientSecret: string;
  redirectUri: string;
  /**
   * Local user to link the federated identity to. The auth flow node sets
   * allowAuthenticationWithoutLocalUser: false (matching real usage, where a user first
   * registers via the vendor and later logs in against that stored identity), so login only
   * succeeds when a local user with this `sub` attribute already exists.
   */
  linkedUser: SocialLinkedUser;
}

export interface SocialLoginSetupResult {
  connectionId: string;
  authFlowId: string;
  applicationId: string;
  userId: string;
  cleanupFunctions: Array<(request: APIRequestContext) => Promise<void>>;
}

export class SocialLoginSetup {
  private readonly profile: VendorProfile;

  constructor(
    private request: APIRequestContext,
    private config: SocialLoginSetupConfig
  ) {
    this.profile = VENDOR_PROFILES[config.vendor];
  }

  /**
   * Perform complete social login setup
   */
  async setup(): Promise<SocialLoginSetupResult> {
    const vendorName = this.profile.displayName;
    console.log(`\n=== ${vendorName} Social Login Setup Started ===`);

    const cleanupFunctions: Array<(request: APIRequestContext) => Promise<void>> = [];

    try {
      const connection = await this.createOrGetConnection();
      if (connection.created) {
        console.log(`✓ ${vendorName} connection created: ${connection.id}`);
        cleanupFunctions.push(request => this.deleteConnection(request, connection.id));
      } else {
        console.log(`✓ Using existing ${vendorName} connection: ${connection.id}`);
      }

      const authFlow = await this.createOrGetAuthFlow(connection.id);
      if (authFlow.created) {
        console.log(`✓ ${vendorName} authentication flow created: ${authFlow.id}`);
        cleanupFunctions.push(request => this.deleteFlow(request, authFlow.id));
      } else {
        console.log(`✓ Using existing ${vendorName} authentication flow: ${authFlow.id}`);
      }

      // recoveryFlowId is cleared for the same reason MFASetup clears it: a leftover recovery
      // flow that calls back into the default authentication flow is rejected as inconsistent
      // once authFlowId points elsewhere. Registration is disabled for the same reason: the
      // backend rejects a registrationFlowId that references a different authFlowId than the
      // one now configured on the application (APP-1039 "Conflicting flow references"), and this
      // setup has no registration flow of its own to keep it pointed at.
      const { appId, originalFlows } = await rewireApplicationFlows(this.request, this.config.appClientId, {
        authFlowId: authFlow.id,
        recoveryFlowId: null,
        registrationFlowId: null,
        isRegistrationFlowEnabled: false,
      });
      console.log(`✓ Application updated with ${vendorName} authentication flow`);
      cleanupFunctions.push(request => restoreApplicationFlows(request, appId, originalFlows));

      const user = await this.createOrGetLinkedUser();
      if (user.created) {
        console.log(`✓ Linked local user created: ${user.id}`);
        cleanupFunctions.push(request => this.deleteUser(request, user.id));
      } else {
        console.log(`✓ Using existing linked local user: ${user.id}`);
      }

      console.log(`=== ${vendorName} Social Login Setup Completed ===\n`);

      return {
        connectionId: connection.id,
        authFlowId: authFlow.id,
        applicationId: appId,
        userId: user.id,
        cleanupFunctions,
      };
    } catch (error) {
      console.error(`✗ ${vendorName} Social Login Setup failed:`, error);
      // A cleanup failure here must not shadow the original setup error, which is the one worth
      // reporting - log it and keep throwing `error` below.
      await SocialLoginSetup.cleanup(this.request, cleanupFunctions).catch(cleanupError => {
        console.error(`✗ ${vendorName} Social Login Setup cleanup also failed:`, cleanupError);
      });
      throw error;
    }
  }

  /**
   * Cleanup all created resources, most-recently-created first.
   *
   * Static, and takes the request context to use: `afterAll` must pass its own live `request`,
   * not the `beforeAll`-scoped one that created the `SocialLoginSetup` instance and closed once
   * `beforeAll` returned. No setup config is needed to tear down.
   *
   * Every cleanup function is attempted even if an earlier one fails, so one broken teardown step
   * doesn't leave the rest of the created resources dangling; any failures are then thrown
   * together so the caller's `afterAll` still surfaces the teardown as failed.
   */
  static async cleanup(
    request: APIRequestContext,
    cleanupFunctions: Array<(request: APIRequestContext) => Promise<void>>
  ): Promise<void> {
    console.log("\n=== Social Login Cleanup Started ===");

    const errors: unknown[] = [];
    for (const cleanupFn of [...cleanupFunctions].reverse()) {
      try {
        await cleanupFn(request);
      } catch (error) {
        console.error("⚠️  Cleanup error:", error);
        errors.push(error);
      }
    }

    console.log("=== Social Login Cleanup Completed ===\n");
    if (errors.length > 0) {
      throw new Error(`Social login cleanup failed for ${errors.length} resource(s): ${errors.join("; ")}`);
    }
  }

  /**
   * Create or get an existing connection pointing at the mock server's client credentials.
   */
  private async createOrGetConnection(): Promise<{ id: string; created: boolean }> {
    const name = `E2E Mock ${this.profile.displayName} Connection`;
    const connectionsApi = new ConnectionsApi(this.request);

    const existing = await connectionsApi.findByName(this.config.vendor, name);
    if (existing) {
      return { id: existing.id, created: false };
    }

    const connection = await connectionsApi.create(this.config.vendor, {
      name,
      description: `Mock ${this.profile.displayName} identity provider for e2e social login testing`,
      clientId: this.config.clientId,
      clientSecret: this.config.clientSecret,
      redirectUri: this.config.redirectUri,
      scopes: this.profile.scopes,
    });
    return { id: connection.id, created: true };
  }

  /**
   * Create or get an existing authentication flow with a "Continue with <vendor>" step
   */
  private async createOrGetAuthFlow(connectionId: string): Promise<{ id: string; created: boolean }> {
    const flowHandle = `e2e-${this.config.vendor}-auth-flow`;
    const flowName = `E2E ${this.profile.displayName} Authentication Flow`;
    const nodes = this.getFlowNodes(connectionId);
    const flowsApi = new FlowsApi(this.request);

    const existing = await flowsApi.findByHandle(flowHandle, "AUTHENTICATION");
    if (existing) {
      // A leftover flow from an earlier run still points its social_auth node at that run's
      // connection id, which may since have been deleted. Overwrite its nodes so the reused
      // flow references the connection just created or found above.
      await flowsApi.update(existing.id, { handle: flowHandle, name: flowName, flowType: "AUTHENTICATION", nodes });
      return { id: existing.id, created: false };
    }

    const created = await flowsApi.create({
      handle: flowHandle,
      name: flowName,
      flowType: "AUTHENTICATION",
      nodes,
    });
    return { id: created.id, created: true };
  }

  /**
   * Create or get a local user carrying the `sub` attribute the auth flow links against.
   */
  private async createOrGetLinkedUser(): Promise<{ id: string; created: boolean }> {
    const usersApi = new UsersApi(this.request);

    const existing = await usersApi.findByUsername(this.config.linkedUser.username);
    if (existing) {
      return { id: existing.id, created: false };
    }

    const user = await usersApi.createUser({
      username: this.config.linkedUser.username,
      email: this.config.linkedUser.email,
      sub: this.config.linkedUser.sub,
    });
    return { id: user.id, created: true };
  }

  /**
   * Delete the connection
   */
  private async deleteConnection(request: APIRequestContext, connectionId: string): Promise<void> {
    const deleted = await new ConnectionsApi(request).deleteById(this.config.vendor, connectionId);
    console.log(
      deleted
        ? `✓ ${this.profile.displayName} connection deleted: ${connectionId}`
        : `⚠️  Could not delete ${this.profile.displayName} connection: ${connectionId}`
    );
  }

  /**
   * Delete the flow
   */
  private async deleteFlow(request: APIRequestContext, flowId: string): Promise<void> {
    const deleted = await new FlowsApi(request).deleteById(flowId);
    console.log(deleted ? `✓ Flow deleted: ${flowId}` : `⚠️  Could not delete flow: ${flowId}`);
  }

  /**
   * Delete the linked user
   */
  private async deleteUser(request: APIRequestContext, userId: string): Promise<void> {
    const deleted = await new UsersApi(request).deleteById(userId);
    console.log(deleted ? `✓ User deleted: ${userId}` : `⚠️  Could not delete user: ${userId}`);
  }

  /**
   * Get the auth flow node definitions with the connection id and vendor branding injected
   */
  private getFlowNodes(connectionId: string): any[] {
    const nodesJson = JSON.stringify(socialAuthFlowNodesTemplate)
      .replace(/\{\{IDP_ID\}\}/g, connectionId)
      .replace(/\{\{EXECUTOR_NAME\}\}/g, this.profile.executorName)
      .replace(/\{\{PROVIDER_LABEL\}\}/g, this.profile.displayName)
      .replace(/\{\{PROVIDER_ICON\}\}/g, this.profile.icon);
    return JSON.parse(nodesJson);
  }
}
