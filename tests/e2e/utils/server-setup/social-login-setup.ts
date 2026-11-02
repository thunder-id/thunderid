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
 * - Local user whose federated identity is linked through a verified linking flow
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
import { serverUrl } from "../api-request";
import { ApplicationsApi } from "../applications-api";
import { UsersApi } from "../users-api";
import { UserTypesApi } from "../user-types-api";
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
  /** Must match the email the vendor's mock server issues: the connection links on email. */
  email: string;
  /** Proves the account while the federated identity is linked to it. */
  password: string;
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
   * links their vendor identity and later logs in with it), so login only succeeds once the
   * identity is linked to this user.
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

      const user = await this.createLinkedUser();
      console.log(`✓ Local user created: ${user.id}`);
      cleanupFunctions.push(request => this.deleteUser(request, user.id));

      await this.recordLink(connection.id);
      console.log(`✓ ${vendorName} identity linked to the local user`);

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
    const connectionData = {
      name,
      description: `Mock ${this.profile.displayName} identity provider for e2e social login testing`,
      clientId: this.config.clientId,
      clientSecret: this.config.clientSecret,
      redirectUri: this.config.redirectUri,
      scopes: this.profile.scopes,
      // recordLink matches the local user on email, the one claim both vendors' mocks share with it.
      attributeConfiguration: {
        userTypeResolution: { default: "person" },
        accountLinking: { attributes: ["email"] },
      },
    };

    const existing = await connectionsApi.findByName(this.config.vendor, name);
    if (existing) {
      // A leftover connection from an earlier run may predate the account linking settings.
      await connectionsApi.update(this.config.vendor, existing.id, connectionData);
      return { id: existing.id, created: false };
    }

    const connection = await connectionsApi.create(this.config.vendor, connectionData);
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
   * Create the local user the federated identity is linked to. A user left over from an earlier
   * run is replaced rather than reused, so its password and links are the ones this run sets.
   */
  private async createLinkedUser(): Promise<{ id: string }> {
    const usersApi = new UsersApi(this.request);
    await usersApi.deleteByUsername(this.config.linkedUser.username);

    const user = await usersApi.createUser({
      username: this.config.linkedUser.username,
      email: this.config.linkedUser.email,
      password: this.config.linkedUser.password,
    });
    return { id: user.id };
  }

  /**
   * Record the (connection, subject) link by running a verified linking flow through the Flow
   * Execution API on a throwaway application. The mock server must already hold the identity.
   */
  private async recordLink(connectionId: string): Promise<void> {
    const handle = `e2e-${this.config.vendor}-link-${Date.now()}`;
    const flowsApi = new FlowsApi(this.request);
    const applicationsApi = new ApplicationsApi(this.request);
    const teardown: Array<() => Promise<boolean>> = [];

    try {
      const verifyFlow = await flowsApi.create({
        handle: `${handle}-verify`,
        name: `E2E Link Verify ${handle}`,
        flowType: "AUTHENTICATION",
        nodes: verifyPasswordFlowNodes(),
      });
      teardown.push(() => flowsApi.deleteById(verifyFlow.id));

      const linkFlow = await flowsApi.create({
        handle,
        name: `E2E Link ${handle}`,
        flowType: "AUTHENTICATION",
        nodes: verifiedLinkingFlowNodes(this.profile.executorName, connectionId, verifyFlow.id),
      });
      teardown.push(() => flowsApi.deleteById(linkFlow.id));

      const registrationFlow = await flowsApi.create({
        handle: `${handle}-reg`,
        name: `E2E Link Registration ${handle}`,
        flowType: "REGISTRATION",
        nodes: isolatedRegistrationFlowNodes(),
      });
      teardown.push(() => flowsApi.deleteById(registrationFlow.id));

      const personType = await new UserTypesApi(this.request).findByHandle("person");
      if (!personType) {
        throw new Error('GET /user-types returned no "person" user type');
      }

      // The token-exchange grant makes the application flow-native, so it is issued a Flow Secret
      // and may start a flow through the Flow Execution API.
      const app = await applicationsApi.create({
        name: `E2E Link App ${handle}`,
        type: "fullstack",
        ouId: personType.ouId,
        authFlowId: linkFlow.id,
        registrationFlowId: registrationFlow.id,
        isRegistrationFlowEnabled: false,
        allowedUserTypes: ["person"],
        inboundAuthConfig: [
          {
            type: "oauth2",
            config: {
              clientId: handle,
              clientSecret: `${handle}-secret`,
              redirectUris: ["https://localhost:3000/callback"],
              grantTypes: ["client_credentials", "urn:ietf:params:oauth:grant-type:token-exchange"],
            },
          },
        ],
      });
      teardown.push(() => applicationsApi.deleteById(app.id));
      const flowSecret = app.flowSecret;
      if (typeof flowSecret !== "string" || !flowSecret) {
        throw new Error("the linking application was not issued a Flow Secret");
      }

      let step = await this.executeFlow({ applicationId: app.id, flowType: "AUTHENTICATION" }, flowSecret);
      const { code, state } = await this.authorizeAtVendor(step.data?.redirectURL);

      step = await this.executeFlow({
        executionId: step.executionId,
        challengeToken: step.challengeToken,
        inputs: { code, state },
      });
      if (step.flowStatus !== "INCOMPLETE" || !step.data?.additionalData?.linkingPromptDetails) {
        throw new Error(`expected the linking prompt for the local user, got ${JSON.stringify(step)}`);
      }

      step = await this.executeFlow({
        executionId: step.executionId,
        challengeToken: step.challengeToken,
        action: LINK_ACCOUNT_ACTION,
      });
      step = await this.executeFlow({
        executionId: step.executionId,
        challengeToken: step.challengeToken,
        inputs: { username: this.config.linkedUser.username, password: this.config.linkedUser.password },
        action: VERIFY_PASSWORD_ACTION,
      });
      if (step.flowStatus !== "COMPLETE") {
        throw new Error(`expected the verified account to be linked, got ${JSON.stringify(step)}`);
      }
    } finally {
      for (const remove of teardown.reverse()) {
        await remove();
      }
    }
  }

  /** One unauthenticated `/flow/execute` call. The Flow Secret is presented only to start a flow. */
  private async executeFlow(body: Record<string, unknown>, flowSecret?: string): Promise<FlowStep> {
    const response = await this.request.post(`${serverUrl}/flow/execute`, {
      data: body,
      headers: flowSecret ? { "Flow-Secret": flowSecret } : {},
      ignoreHTTPSErrors: true,
    });
    if (!response.ok()) {
      throw new Error(`POST /flow/execute failed (${response.status()}): ${await response.text()}`);
    }
    return (await response.json()) as FlowStep;
  }

  /** Follow the flow's redirect to the vendor mock, which approves at once and returns a code. */
  private async authorizeAtVendor(redirectURL: string | undefined): Promise<{ code: string; state: string }> {
    if (!redirectURL) {
      throw new Error("expected the linking flow to redirect to the vendor");
    }
    const response = await this.request.get(redirectURL, { maxRedirects: 0, ignoreHTTPSErrors: true });
    const location = response.headers()["location"];
    if (!location) {
      throw new Error(`expected the vendor to redirect back with a code, got ${response.status()}`);
    }
    const params = new URL(location).searchParams;
    const code = params.get("code");
    if (!code) {
      throw new Error(`the vendor redirected back without a code: ${location}`);
    }
    return { code, state: params.get("state") ?? "" };
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
   * Delete the linked user, along with its link
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

type FlowStep = {
  executionId: string;
  flowStatus: string;
  challengeToken?: string;
  data?: { redirectURL?: string; additionalData?: Record<string, string> };
};

// Action refs of the prompts the verified linking flow forwards a matched account through.
const LINK_ACCOUNT_ACTION = "link_account";
const VERIFY_PASSWORD_ACTION = "verify_password";
const SEPARATE_ACCOUNT_ACTION = "use_new_account";

const usernameInput = { identifier: "username", type: "TEXT_INPUT", required: true };
const passwordInput = { identifier: "password", type: "PASSWORD_INPUT", required: true };

/** A username and password sign-in, run as its own frame by the linking flow's CALL node. */
function verifyPasswordFlowNodes(): unknown[] {
  return [
    { id: "start", type: "START", onSuccess: "credentials_prompt" },
    {
      id: "credentials_prompt",
      type: "PROMPT",
      prompts: [
        {
          inputs: [usernameInput, passwordInput],
          action: { ref: VERIFY_PASSWORD_ACTION, nextNode: "credentials_auth" },
        },
      ],
    },
    {
      id: "credentials_auth",
      type: "TASK_EXECUTION",
      executor: { name: "CredentialsAuthExecutor", inputs: [usernameInput, passwordInput] },
      onSuccess: "auth_assert",
      onIncomplete: "credentials_prompt",
    },
    { id: "auth_assert", type: "TASK_EXECUTION", executor: { name: "AuthAssertExecutor" }, onSuccess: "end" },
    { id: "end", type: "END" },
  ];
}

/**
 * Federated sign-in followed by the linking node: a matched account is confirmed, verified through
 * the called flow, and linked when the verification returns to the linking node.
 */
function verifiedLinkingFlowNodes(executorName: string, idpId: string, verifyFlowId: string): unknown[] {
  return [
    { id: "start", type: "START", onSuccess: "federated_auth" },
    {
      id: "federated_auth",
      type: "TASK_EXECUTION",
      properties: { idpId },
      executor: { name: executorName },
      onSuccess: "linking",
    },
    {
      id: "linking",
      type: "TASK_EXECUTION",
      executor: { name: "AccountLinkingExecutor" },
      onSuccess: "auth_assert",
      onIncomplete: "linking_prompt",
    },
    {
      id: "linking_prompt",
      type: "PROMPT",
      prompts: [
        { action: { ref: LINK_ACCOUNT_ACTION, type: "CONFIRM", nextNode: "verify_account" } },
        { action: { ref: SEPARATE_ACCOUNT_ACTION, type: "REJECT", nextNode: "linking" } },
      ],
    },
    { id: "verify_account", type: "CALL", flow: { ref: verifyFlowId }, onSuccess: "linking" },
    { id: "auth_assert", type: "TASK_EXECUTION", executor: { name: "AuthAssertExecutor" }, onSuccess: "end" },
    { id: "end", type: "END" },
  ];
}

/** A registration flow of the linking application's own, so it references no other application's flows. */
function isolatedRegistrationFlowNodes(): unknown[] {
  return [
    { id: "start", type: "START", onSuccess: "user_type_resolver" },
    {
      id: "user_type_resolver",
      type: "TASK_EXECUTION",
      executor: { name: "UserTypeResolver" },
      onSuccess: "provisioning",
    },
    { id: "provisioning", type: "TASK_EXECUTION", executor: { name: "ProvisioningExecutor" }, onSuccess: "end" },
    { id: "end", type: "END" },
  ];
}
