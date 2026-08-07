// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * MFA Setup Utilities
 *
 * Automated setup for MFA testing prerequisites:
 * - Notification sender creation
 * - MFA flow creation
 * - Test user creation
 * - Application configuration
 *
 * All backend calls go through the `*Api` helpers (which themselves go through `send`/`sendOk`,
 * owning the admin bearer token and `ignoreHTTPSErrors`), so nothing here handles auth headers
 * directly. Resources are looked up by name/handle before creating - the notification sender and
 * both flows are shared, name-addressed resources, and the sender's URL is a server-wide setting,
 * so nothing may run this setup concurrently with itself. The calling spec is therefore restricted
 * to a single browser project (see SERVER_STATE_SPECS in playwright.config.ts). It does not
 * contend with social login or default login: MFA rewires its own dedicated application
 * (constants/sample-apps.ts) instead of sharing `REACT_SDK_SAMPLE`.
 */

import type { APIRequestContext } from "@playwright/test";
import { UsersApi } from "../users-api";
import { ConnectionsApi } from "../connections-api";
import { FlowsApi } from "../flows-api";
import { rewireApplicationFlows, restoreApplicationFlows } from "./application-flows";
import mfaFlowNodesTemplate from "./mfa-flow-nodes.json";
import mfaRegistrationFlowNodesTemplate from "./mfa-registration-flow-nodes.json";

export interface SetupConfig {
  /** clientId of the application to rewire (constants/sample-apps.ts). */
  clientId: string;
  mockSmsUrl: string;
  testUser: {
    username: string;
    password: string;
    email: string;
    mobile_number: string;
    given_name: string;
  };
}

export interface SetupResult {
  notificationSenderId: string;
  authFlowId: string;
  registrationFlowId: string;
  userId: string;
  applicationId: string;
  cleanupFunctions: Array<(request: APIRequestContext) => Promise<void>>;
}

export class MFASetup {
  constructor(
    private request: APIRequestContext,
    private config: SetupConfig
  ) {}

  /**
   * Perform complete MFA setup
   */
  async setup(): Promise<SetupResult> {
    console.log("\n=== MFA Setup Started ===");

    const cleanupFunctions: Array<(request: APIRequestContext) => Promise<void>> = [];

    try {
      // Step 1: Create notification sender
      const sender = await this.createOrGetNotificationSender();
      if (sender.created) {
        console.log(`✓ Notification sender created: ${sender.id}`);
        cleanupFunctions.push(request => this.deleteNotificationSender(request, sender.id));
      } else {
        console.log(`✓ Using existing notification sender: ${sender.id}`);
      }

      // Step 2: Create MFA registration flow. It is created before the authentication flow because
      // the authentication flow's call_registration node has to reference this flow's id.
      const regFlow = await this.createOrGetMFARegistrationFlow();
      if (regFlow.created) {
        console.log(`✓ MFA registration flow created: ${regFlow.id}`);
        cleanupFunctions.push(request => this.deleteFlow(request, regFlow.id));
      } else {
        console.log(`✓ Using existing MFA registration flow: ${regFlow.id}`);
      }

      // Step 3: Create MFA authentication flow
      const authFlow = await this.createOrGetMFAAuthFlow(sender.id, regFlow.id);
      if (authFlow.created) {
        console.log(`✓ MFA authentication flow created: ${authFlow.id}`);
        cleanupFunctions.push(request => this.deleteFlow(request, authFlow.id));
      } else {
        console.log(`✓ Using existing MFA authentication flow: ${authFlow.id}`);
      }

      // Step 4: Create test user
      const user = await this.createOrGetTestUser();
      if (user.created) {
        console.log(`✓ Test user created: ${user.id}`);
        cleanupFunctions.push(request => this.deleteUser(request, user.id));
      } else {
        console.log(`✓ Using existing test user: ${user.id}`);
      }

      // Step 5: Update application with MFA flows
      const { appId, originalFlows } = await rewireApplicationFlows(this.request, this.config.clientId, {
        authFlowId: authFlow.id,
        registrationFlowId: regFlow.id,
        recoveryFlowId: null,
        isRegistrationFlowEnabled: true,
      });
      console.log(`✓ Application updated with MFA flows`);
      cleanupFunctions.push(request => restoreApplicationFlows(request, appId, originalFlows));
      console.log("=== MFA Setup Completed ===\n");

      return {
        notificationSenderId: sender.id,
        authFlowId: authFlow.id,
        registrationFlowId: regFlow.id,
        userId: user.id,
        applicationId: appId,
        cleanupFunctions,
      };
    } catch (error) {
      console.error("✗ MFA Setup failed:", error);
      // Run cleanup for any resources created before failure
      await MFASetup.cleanup(this.request, cleanupFunctions);
      throw error;
    }
  }

  /**
   * Cleanup all created resources, most-recently-created first.
   *
   * Static, and takes the request context to use: `afterAll` must pass its own live `request`,
   * not the `beforeAll`-scoped one that created the `MFASetup` instance and closed once
   * `beforeAll` returned. No setup config is needed to tear down.
   */
  static async cleanup(
    request: APIRequestContext,
    cleanupFunctions: Array<(request: APIRequestContext) => Promise<void>>
  ): Promise<void> {
    console.log("\n=== MFA Cleanup Started ===");

    for (const cleanupFn of [...cleanupFunctions].reverse()) {
      try {
        await cleanupFn(request);
      } catch (error) {
        console.error("⚠️  Cleanup error (non-fatal):", error);
      }
    }

    console.log("=== MFA Cleanup Completed ===\n");
  }

  /**
   * Create or get existing notification sender for SMS
   */
  private async createOrGetNotificationSender(): Promise<{ id: string; created: boolean }> {
    const senderName = "E2E Mock SMS Sender";
    const connectionsApi = new ConnectionsApi(this.request);

    const existing = await connectionsApi.findByName("sms-gateway", senderName);
    if (existing) {
      // A sender left over from an earlier run may point at a stale mock SMS URL (e.g. a
      // different MOCK_SMS_SERVER_PORT, or a run that crashed before its own cleanup ran).
      // Reconcile it so this run's mock server actually receives the webhook.
      const detail = await connectionsApi.get("sms-gateway", existing.id);
      if (detail.url !== this.config.mockSmsUrl) {
        await connectionsApi.update("sms-gateway", existing.id, {
          name: senderName,
          description: "Mock SMS sender for e2e MFA testing",
          url: this.config.mockSmsUrl,
          httpMethod: "POST",
          contentType: "JSON",
        });
      }
      return { id: existing.id, created: false };
    }

    const sender = await connectionsApi.create("sms-gateway", {
      name: senderName,
      description: "Mock SMS sender for e2e MFA testing",
      url: this.config.mockSmsUrl,
      httpMethod: "POST",
      contentType: "JSON",
    });
    return { id: sender.id, created: true };
  }

  /**
   * Create or get existing MFA authentication flow
   */
  private async createOrGetMFAAuthFlow(
    senderId: string,
    registrationFlowId: string
  ): Promise<{ id: string; created: boolean }> {
    const flowHandle = "e2e-mfa-auth-flow";
    const flowName = "E2E MFA Authentication Flow";
    const nodes = this.getMFAFlowNodes(senderId, registrationFlowId);
    const flowsApi = new FlowsApi(this.request);

    const existing = await flowsApi.findByHandle(flowHandle, "AUTHENTICATION");
    if (existing) {
      // A leftover flow from an earlier run still points its call_registration node at that run's
      // registration flow, which no longer matches the one just created. Overwrite its nodes so the
      // reused flow references the current registration flow.
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
   * Create or get existing MFA registration flow
   */
  private async createOrGetMFARegistrationFlow(): Promise<{ id: string; created: boolean }> {
    const flowHandle = "e2e-mfa-reg-flow";
    const flowsApi = new FlowsApi(this.request);

    const existing = await flowsApi.findByHandle(flowHandle, "REGISTRATION");
    if (existing) {
      return { id: existing.id, created: false };
    }

    const created = await flowsApi.create({
      handle: flowHandle,
      name: "E2E MFA Registration Flow",
      flowType: "REGISTRATION",
      nodes: this.getMFARegistrationFlowNodes(),
    });
    return { id: created.id, created: true };
  }

  /**
   * Create or get existing test user with mobile number
   */
  private async createOrGetTestUser(): Promise<{ id: string; created: boolean }> {
    const usersApi = new UsersApi(this.request);

    const existing = await usersApi.findByUsername(this.config.testUser.username);
    if (existing) {
      return { id: existing.id, created: false };
    }

    const user = await usersApi.createUser({
      username: this.config.testUser.username,
      password: this.config.testUser.password,
      given_name: this.config.testUser.given_name,
      email: this.config.testUser.email,
      mobile_number: this.config.testUser.mobile_number,
    });
    return { id: user.id, created: true };
  }

  /**
   * Delete the test user
   */
  private async deleteUser(request: APIRequestContext, userId: string): Promise<void> {
    const deleted = await new UsersApi(request).deleteById(userId);
    console.log(deleted ? `✓ User deleted: ${userId}` : `⚠️  Could not delete user: ${userId}`);
  }

  /**
   * Delete notification sender
   */
  private async deleteNotificationSender(request: APIRequestContext, senderId: string): Promise<void> {
    const deleted = await new ConnectionsApi(request).deleteById("sms-gateway", senderId);
    console.log(
      deleted ? `✓ Notification sender deleted: ${senderId}` : `⚠️  Could not delete notification sender: ${senderId}`
    );
  }

  /**
   * Delete flow
   */
  private async deleteFlow(request: APIRequestContext, flowId: string): Promise<void> {
    const deleted = await new FlowsApi(request).deleteById(flowId);
    console.log(deleted ? `✓ Flow deleted: ${flowId}` : `⚠️  Could not delete flow: ${flowId}`);
  }

  /**
   * Get MFA flow node definitions with senderId and registration flow id injected.
   *
   * The auth flow's `call_registration` node must target the same registration flow the application
   * is bound to, otherwise the server rejects the application update with APP-1039
   * ("Conflicting flow references"), so the id is templated in rather than hardcoded.
   */
  private getMFAFlowNodes(senderId: string, registrationFlowId: string): any[] {
    // Deep clone the template and replace the placeholders
    const nodesJson = JSON.stringify(mfaFlowNodesTemplate);
    const nodesWithPlaceholders = nodesJson
      .replace(/\{\{SENDER_ID\}\}/g, senderId)
      .replace(/\{\{REGISTRATION_FLOW_ID\}\}/g, registrationFlowId);
    return JSON.parse(nodesWithPlaceholders);
  }

  /**
   * Get MFA registration flow node definitions
   */
  private getMFARegistrationFlowNodes(): any[] {
    return mfaRegistrationFlowNodesTemplate;
  }
}
