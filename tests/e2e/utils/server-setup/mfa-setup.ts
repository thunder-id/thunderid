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
 * All backend calls go through `send`/`sendOk`, which own the admin bearer token and
 * `ignoreHTTPSErrors`, so nothing here handles auth headers directly.
 */

import type { APIRequestContext } from "@playwright/test";
import { send, sendOk } from "../api-request";
import { UsersApi } from "../users-api";
import { ApplicationsApi } from "../applications-api";
import mfaFlowNodesTemplate from "./mfa-flow-nodes.json";
import mfaRegistrationFlowNodesTemplate from "./mfa-registration-flow-nodes.json";

export interface SetupConfig {
  mockSmsUrl: string;
  testUser: {
    username: string;
    password: string;
    email: string;
    mobile_number: string;
    given_name: string;
  };
}

type OriginalFlows = {
  authFlowId: string | undefined;
  registrationFlowId: string | undefined;
  recoveryFlowId: string | null;
  isRegistrationFlowEnabled: boolean | undefined;
};

export interface SetupResult {
  notificationSenderId: string;
  authFlowId: string;
  registrationFlowId: string;
  userId: string;
  applicationId: string;
  cleanupFunctions: Array<(request: APIRequestContext) => Promise<void>>;
  resourcesCreated: {
    notificationSender: boolean;
    authFlow: boolean;
    registrationFlow: boolean;
    user: boolean;
  };
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
    const resourcesCreated = {
      notificationSender: false,
      authFlow: false,
      registrationFlow: false,
      user: false,
    };

    try {
      // Step 1: Create notification sender
      const sender = await this.createOrGetNotificationSender();
      if (sender.created) {
        console.log(`✓ Notification sender created: ${sender.id}`);
        cleanupFunctions.push(request => this.deleteNotificationSender(request, sender.id));
        resourcesCreated.notificationSender = true;
      } else {
        console.log(`✓ Using existing notification sender: ${sender.id}`);
      }

      // Step 2: Create MFA registration flow. It is created before the authentication flow because
      // the authentication flow's call_registration node has to reference this flow's id.
      const regFlow = await this.createOrGetMFARegistrationFlow();
      if (regFlow.created) {
        console.log(`✓ MFA registration flow created: ${regFlow.id}`);
        cleanupFunctions.push(request => this.deleteFlow(request, regFlow.id));
        resourcesCreated.registrationFlow = true;
      } else {
        console.log(`✓ Using existing MFA registration flow: ${regFlow.id}`);
      }

      // Step 3: Create MFA authentication flow
      const authFlow = await this.createOrGetMFAAuthFlow(sender.id, regFlow.id);
      if (authFlow.created) {
        console.log(`✓ MFA authentication flow created: ${authFlow.id}`);
        cleanupFunctions.push(request => this.deleteFlow(request, authFlow.id));
        resourcesCreated.authFlow = true;
      } else {
        console.log(`✓ Using existing MFA authentication flow: ${authFlow.id}`);
      }

      // Step 4: Create test user
      const user = await this.createOrGetTestUser();
      if (user.created) {
        console.log(`✓ Test user created: ${user.id}`);
        cleanupFunctions.push(request => this.deleteUser(request, user.id));
        resourcesCreated.user = true;
      } else {
        console.log(`✓ Using existing test user: ${user.id}`);
      }

      // Step 5: Update application with MFA flows
      const { appId, originalFlows } = await this.updateApplicationFlows(authFlow.id, regFlow.id);
      console.log(`✓ Application updated with MFA flows`);
      cleanupFunctions.push(request => this.revertApplicationFlows(request, appId, originalFlows));
      console.log("=== MFA Setup Completed ===\n");

      return {
        notificationSenderId: sender.id,
        authFlowId: authFlow.id,
        registrationFlowId: regFlow.id,
        userId: user.id,
        applicationId: appId,
        cleanupFunctions,
        resourcesCreated,
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

    const response = await send(this.request, "POST", "/connections/sms-gateway", {
      name: senderName,
      description: "Mock SMS sender for e2e MFA testing",
      url: this.config.mockSmsUrl,
      httpMethod: "POST",
      contentType: "JSON",
    });

    if (response.ok()) {
      const data = await response.json();
      return { id: data.id, created: true };
    }

    // Check if it's a duplicate error
    const errorText = await response.text();
    if (errorText.includes("MNS-1005") || errorText.includes("Duplicate sender name")) {
      return { id: await this.getExistingNotificationSender(senderName), created: false };
    }

    throw new Error(`Failed to create notification sender: ${errorText}`);
  }

  /**
   * Get existing notification sender by name
   */
  private async getExistingNotificationSender(name: string): Promise<string> {
    const response = await sendOk(this.request, "GET", "/connections/sms-gateway");
    const data = await response.json();
    const sender = data?.find((s: any) => s.name == name);

    if (!sender) {
      console.log(data);
      throw new Error(`Notification sender '${name}' exists but could not be found in the list`);
    }

    return sender.id;
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

    const response = await send(this.request, "POST", "/flows", {
      handle: flowHandle,
      name: flowName,
      flowType: "AUTHENTICATION",
      activeVersion: 3,
      nodes,
    });

    if (response.ok()) {
      const data = await response.json();
      return { id: data.id, created: true };
    }

    // Check if it's a duplicate error
    const errorText = await response.text();
    if (errorText.includes("duplicate") || errorText.includes("already exists") || response.status() === 409) {
      const existingId = await this.getExistingFlow(flowHandle, "AUTHENTICATION");
      // A leftover flow from an earlier run still points its call_registration node at that run's
      // registration flow, which no longer matches the one just created. Overwrite its nodes so the
      // reused flow references the current registration flow.
      await this.updateFlowNodes(existingId, flowHandle, flowName, "AUTHENTICATION", nodes);
      return { id: existingId, created: false };
    }

    throw new Error(`Failed to create MFA authentication flow: ${errorText}`);
  }

  /**
   * Overwrite an existing flow's node definitions.
   */
  private async updateFlowNodes(
    flowId: string,
    handle: string,
    name: string,
    flowType: string,
    nodes: any[]
  ): Promise<void> {
    await sendOk(this.request, "PUT", `/flows/${flowId}`, { handle, name, flowType, nodes });
  }

  /**
   * Create or get existing MFA registration flow
   */
  private async createOrGetMFARegistrationFlow(): Promise<{ id: string; created: boolean }> {
    const flowHandle = "e2e-mfa-reg-flow";

    const response = await send(this.request, "POST", "/flows", {
      handle: flowHandle,
      name: "E2E MFA Registration Flow",
      flowType: "REGISTRATION",
      nodes: this.getMFARegistrationFlowNodes(),
    });

    if (response.ok()) {
      const data = await response.json();
      return { id: data.id, created: true };
    }

    // Check if it's a duplicate error
    const errorText = await response.text();
    if (errorText.includes("duplicate") || errorText.includes("already exists") || response.status() === 409) {
      return { id: await this.getExistingFlow(flowHandle, "REGISTRATION"), created: false };
    }

    throw new Error(`Failed to create MFA registration flow: ${errorText}`);
  }

  /**
   * Get existing flow by handle
   */
  private async getExistingFlow(handle: string, flowType?: string): Promise<string> {
    let filterQuery = `handle eq "${handle}"`;
    if (flowType) {
      filterQuery += ` and flowType eq "${flowType}"`;
    }

    const response = await sendOk(this.request, "GET", `/flows?filter=${encodeURIComponent(filterQuery)}`);
    const data = await response.json();
    const flow = flowType
      ? data.flows?.find((f: any) => f.handle === handle && f.flowType === flowType)
      : data.flows?.find((f: any) => f.handle === handle);

    if (!flow) {
      throw new Error(
        `Flow '${handle}' ${flowType ? `with type '${flowType}'` : ""} exists but could not be found in the list`
      );
    }

    return flow.id;
  }

  /**
   * Create or get existing test user with mobile number
   */
  private async createOrGetTestUser(): Promise<{ id: string; created: boolean }> {
    const usersApi = new UsersApi(this.request);

    try {
      const user = await usersApi.createUser({
        username: this.config.testUser.username,
        password: this.config.testUser.password,
        given_name: this.config.testUser.given_name,
        email: this.config.testUser.email,
        mobile_number: this.config.testUser.mobile_number,
      });
      return { id: user.id, created: true };
    } catch (error) {
      // User might already exist from an earlier run.
      const message = String(error);
      if (!message.includes("409") && !message.includes("already exists")) throw error;

      const existing = await usersApi.findByUsername(this.config.testUser.username);
      if (!existing) throw new Error("Test user already exists but could not be found");
      return { id: existing.id, created: false };
    }
  }

  /**
   * Delete the test user
   */
  private async deleteUser(request: APIRequestContext, userId: string): Promise<void> {
    const deleted = await new UsersApi(request).deleteById(userId);
    console.log(deleted ? `✓ User deleted: ${userId}` : `⚠️  Could not delete user: ${userId}`);
  }

  /**
   * Update application with MFA authentication and registration flows.
   * Returns the app id together with its prior flow bindings
   */
  private async updateApplicationFlows(
    authFlowId: string,
    registrationFlowId: string
  ): Promise<{ appId: string; originalFlows: OriginalFlows }> {
    if (!authFlowId || !registrationFlowId) {
      throw new Error(`Cannot update application flows: missing ${!authFlowId ? "authFlowId" : "registrationFlowId"}`);
    }

    const applicationsApi = new ApplicationsApi(this.request);
    const targetApp = await applicationsApi.findByClientId("REACT_SDK_SAMPLE");
    if (!targetApp) {
      throw new Error(`Application with clientId "REACT_SDK_SAMPLE" not found`);
    }

    const appData = await applicationsApi.get(targetApp.id);
    const originalFlows: OriginalFlows = {
      authFlowId: appData.authFlowId,
      registrationFlowId: appData.registrationFlowId,
      recoveryFlowId: appData.recoveryFlowId ?? null,
      isRegistrationFlowEnabled: appData.isRegistrationFlowEnabled,
    };

    // Update with new flow IDs. recoveryFlowId is cleared to avoid conflicts
    // with MFA registration flow, and isRegistrationFlowEnabled is set to true.
    await applicationsApi.update(targetApp.id, {
      ...appData,
      authFlowId,
      registrationFlowId,
      recoveryFlowId: null,
      isRegistrationFlowEnabled: true,
    });

    return { appId: targetApp.id, originalFlows };
  }

  /**
   * Restore an application's flow bindings to what they were before MFA setup rewired them.
   */
  private async revertApplicationFlows(
    request: APIRequestContext,
    appId: string,
    originalFlows: OriginalFlows
  ): Promise<void> {
    try {
      const applicationsApi = new ApplicationsApi(request);
      const appData = await applicationsApi.get(appId);
      await applicationsApi.update(appId, { ...appData, ...originalFlows });
      console.log(`✓ Application flows reverted: ${appId}`);
    } catch (error) {
      console.log(`⚠️  Error reverting application flows: ${error}`);
    }
  }

  /**
   * Delete notification sender
   */
  private async deleteNotificationSender(request: APIRequestContext, senderId: string): Promise<void> {
    const response = await send(request, "DELETE", `/connections/sms-gateway/${senderId}`);
    console.log(
      response.ok() ? `✓ Notification sender deleted: ${senderId}` : `⚠️  Could not delete notification sender: ${senderId}`
    );
  }

  /**
   * Delete flow
   */
  private async deleteFlow(request: APIRequestContext, flowId: string): Promise<void> {
    const response = await send(request, "DELETE", `/flows/${flowId}`);
    console.log(response.ok() ? `✓ Flow deleted: ${flowId}` : `⚠️  Could not delete flow: ${flowId}`);
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
