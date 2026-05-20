/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/**
 * MFA Setup Utilities
 *
 * Automated setup for MFA testing prerequisites:
 * - Admin authentication
 * - Notification sender creation
 * - MFA flow creation
 * - Test user creation
 * - Application configuration
 */

import { APIRequestContext, request as playwrightRequest } from "@playwright/test";
import mfaFlowNodesTemplate from "./mfa-flow-nodes.json";
import mfaRegistrationFlowNodesTemplate from "./mfa-registration-flow-nodes.json";

export interface SetupConfig {
  serverUrl: string;
  mockSmsUrl: string;
  adminUsername: string;
  adminPassword: string;
  applicationId: string;
  testUser: {
    username: string;
    password: string;
    email: string;
    mobileNumber: string;
    given_name: string;
  };
}

export interface SetupResult {
  adminToken: string;
  notificationSenderId: string;
  authFlowId: string;
  registrationFlowId: string;
  userId: string;
  applicationId: string;
  cleanupFunctions: Array<() => Promise<void>>;
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

    const cleanupFunctions: Array<() => Promise<void>> = [];
    const resourcesCreated = {
      notificationSender: false,
      authFlow: false,
      registrationFlow: false,
      user: false,
    };

    try {
      // Step 1: Get admin token
      const adminToken = await this.getAdminToken();
      console.log("✓ Admin authentication successful");

      // Step 2: Create notification sender
      const notificationSenderId = await this.createOrGetNotificationSender(adminToken);
      if (notificationSenderId.startsWith("created:")) {
        const id = notificationSenderId.replace("created:", "");
        console.log(`✓ Notification sender created: ${id}`);
        cleanupFunctions.push(() => this.deleteNotificationSender(adminToken, id));
        resourcesCreated.notificationSender = true;
      } else {
        console.log(`✓ Using existing notification sender: ${notificationSenderId}`);
      }
      const senderId = notificationSenderId.replace("created:", "");

      // Step 3: Create MFA authentication flow
      const authFlowId = await this.createOrGetMFAAuthFlow(adminToken, senderId);
      if (authFlowId.startsWith("created:")) {
        const id = authFlowId.replace("created:", "");
        console.log(`✓ MFA authentication flow created: ${id}`);
        cleanupFunctions.push(() => this.deleteFlow(adminToken, id));
        resourcesCreated.authFlow = true;
      } else {
        console.log(`✓ Using existing MFA authentication flow: ${authFlowId}`);
      }
      const actualAuthFlowId = authFlowId.replace("created:", "");

      // Step 4: Create MFA registration flow
      const regFlowId = await this.createOrGetMFARegistrationFlow(adminToken);
      if (regFlowId.startsWith("created:")) {
        const id = regFlowId.replace("created:", "");
        console.log(`✓ MFA registration flow created: ${id}`);
        cleanupFunctions.push(() => this.deleteFlow(adminToken, id));
        resourcesCreated.registrationFlow = true;
      } else {
        console.log(`✓ Using existing MFA registration flow: ${regFlowId}`);
      }
      const actualRegFlowId = regFlowId.replace("created:", "");

      // Step 5: Create test user
      const userResult = await this.createOrGetTestUser(adminToken);
      if (userResult.startsWith("created:")) {
        const id = userResult.replace("created:", "");
        console.log(`✓ Test user created: ${id}`);
        cleanupFunctions.push(() => this.deleteUser(adminToken, id));
        resourcesCreated.user = true;
      } else {
        console.log(`✓ Using existing test user: ${userResult}`);
      }
      const userId = userResult.replace("created:", "");

      // Step 6: Update application with MFA flows
      const actualAppId = await this.updateApplicationFlows(adminToken, actualAuthFlowId, actualRegFlowId);
      console.log(`✓ Application updated with MFA flows`);
      console.log("=== MFA Setup Completed ===\n");

      return {
        adminToken,
        notificationSenderId: senderId,
        authFlowId: actualAuthFlowId,
        registrationFlowId: actualRegFlowId,
        userId,
        applicationId: actualAppId,
        cleanupFunctions,
        resourcesCreated,
      };
    } catch (error) {
      console.error("✗ MFA Setup failed:", error);
      // Run cleanup for any resources created before failure
      await this.cleanup(cleanupFunctions);
      throw error;
    }
  }

  /**
   * Cleanup all created resources
   */
  async cleanup(cleanupFunctions: Array<() => Promise<void>>): Promise<void> {
    console.log("\n=== MFA Cleanup Started ===");

    for (const cleanup of cleanupFunctions.reverse()) {
      try {
        await cleanup();
      } catch (error) {
        console.error("⚠️  Cleanup error (non-fatal):", error);
      }
    }

    console.log("=== MFA Cleanup Completed ===\n");
  }

  /**
   * Get admin authentication token
   */
  private async getAdminToken(): Promise<string> {
    // Step 1: Start authentication flow
    const flowResponse = await this.request.post(`${this.config.serverUrl}/flow/execute`, {
      data: {
        applicationId: this.config.applicationId,
        flowType: "AUTHENTICATION",
      },
      ignoreHTTPSErrors: true,
    });

    if (!flowResponse.ok()) {
      throw new Error(`Failed to start authentication flow: ${await flowResponse.text()}`);
    }

    const flowData = await flowResponse.json();
    const executionId = flowData.executionId;
    const challengeToken = flowData.challengeToken;

    // Step 2: Submit credentials
    const authResponse = await this.request.post(`${this.config.serverUrl}/flow/execute`, {
      data: {
        executionId: executionId,
        challengeToken: challengeToken,
        inputs: {
          username: this.config.adminUsername,
          password: this.config.adminPassword,
          requested_permissions: "system",
        },
        action: "action_001",
      },
      ignoreHTTPSErrors: true,
    });

    if (!authResponse.ok()) {
      throw new Error(`Admin authentication failed: ${await authResponse.text()}`);
    }

    const authData = await authResponse.json();
    return authData.assertion;
  }

  /**
   * Create or get existing notification sender for SMS
   */
  private async createOrGetNotificationSender(adminToken: string): Promise<string> {
    const senderName = "E2E Mock SMS Sender";

    // Try to create the notification sender
    const response = await this.request.post(`${this.config.serverUrl}/notification-senders/message`, {
      data: {
        name: senderName,
        description: "Mock SMS sender for e2e MFA testing",
        provider: "custom",
        properties: [
          {
            name: "url",
            value: this.config.mockSmsUrl,
            isSecret: false,
          },
          {
            name: "http_method",
            value: "POST",
            isSecret: false,
          },
          {
            name: "content_type",
            value: "JSON",
            isSecret: false,
          },
        ],
      },
      headers: {
        Authorization: `Bearer ${adminToken}`,
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      ignoreHTTPSErrors: true,
    });

    if (response.ok()) {
      const data = await response.json();
      return `created:${data.id}`;
    }

    // Check if it's a duplicate error
    const errorText = await response.text();
    if (errorText.includes("MNS-1005") || errorText.includes("Duplicate sender name")) {
      const existingId = await this.getExistingNotificationSender(adminToken, senderName);
      return existingId; // Return without "created:" prefix
    }

    throw new Error(`Failed to create notification sender: ${errorText}`);
  }

  /**
   * Get existing notification sender by name
   */
  private async getExistingNotificationSender(adminToken: string, name: string): Promise<string> {
    const response = await this.request.get(`${this.config.serverUrl}/notification-senders/message`, {
      headers: {
        Authorization: `Bearer ${adminToken}`,
      },
      ignoreHTTPSErrors: true,
    });

    if (!response.ok()) {
      throw new Error(`Failed to fetch notification senders: ${await response.text()}`);
    }

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
  private async createOrGetMFAAuthFlow(adminToken: string, senderId: string): Promise<string> {
    const flowHandle = "e2e-mfa-auth-flow";

    const response = await this.request.post(`${this.config.serverUrl}/flows`, {
      data: {
        handle: flowHandle,
        name: "E2E MFA Authentication Flow",
        flowType: "AUTHENTICATION",
        activeVersion: 3,
        nodes: this.getMFAFlowNodes(senderId),
      },
      headers: {
        Authorization: `Bearer ${adminToken}`,
        "Content-Type": "application/json",
      },
      ignoreHTTPSErrors: true,
    });

    if (response.ok()) {
      const data = await response.json();
      return `created:${data.id}`;
    }

    // Check if it's a duplicate error
    const errorText = await response.text();
    if (errorText.includes("duplicate") || errorText.includes("already exists") || response.status() === 409) {
      const existingId = await this.getExistingFlow(adminToken, flowHandle);
      return existingId; // Return without "created:" prefix
    }

    throw new Error(`Failed to create MFA authentication flow: ${errorText}`);
  }

  /**
   * Create or get existing MFA registration flow
   */
  private async createOrGetMFARegistrationFlow(adminToken: string): Promise<string> {
    const flowHandle = "e2e-mfa-reg-flow";

    const response = await this.request.post(`${this.config.serverUrl}/flows`, {
      data: {
        handle: flowHandle,
        name: "E2E MFA Registration Flow",
        flowType: "REGISTRATION",
        activeVersion: 2,
        nodes: this.getMFARegistrationFlowNodes(),
      },
      headers: {
        Authorization: `Bearer ${adminToken}`,
        "Content-Type": "application/json",
      },
      ignoreHTTPSErrors: true,
    });

    if (response.ok()) {
      const data = await response.json();
      return `created:${data.id}`;
    }

    // Check if it's a duplicate error
    const errorText = await response.text();
    if (errorText.includes("duplicate") || errorText.includes("already exists") || response.status() === 409) {
      const existingId = await this.getExistingFlow(adminToken, flowHandle, "REGISTRATION");
      return existingId; // Return without "created:" prefix
    }

    throw new Error(`Failed to create MFA registration flow: ${errorText}`);
  }

  /**
   * Get existing flow by handle
   */
  private async getExistingFlow(adminToken: string, handle: string, flowType?: string): Promise<string> {
    let filterQuery = `handle eq "${handle}"`;
    if (flowType) {
      filterQuery += ` and flowType eq "${flowType}"`;
    }

    const response = await this.request.get(
      `${this.config.serverUrl}/flows?filter=${encodeURIComponent(filterQuery)}`,
      {
        headers: {
          Authorization: `Bearer ${adminToken}`,
        },
        ignoreHTTPSErrors: true,
      }
    );

    if (!response.ok()) {
      throw new Error(`Failed to fetch flows: ${await response.text()}`);
    }

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
  private async createOrGetTestUser(adminToken: string): Promise<string> {
    // Get organization unit from Person user type
    const schemasResponse = await this.request.get(`${this.config.serverUrl}/user-types`, {
      headers: {
        Authorization: `Bearer ${adminToken}`,
      },
      ignoreHTTPSErrors: true,
    });

    if (!schemasResponse.ok()) {
      throw new Error(`Failed to fetch user types: ${await schemasResponse.text()}`);
    }

    const schemasData = await schemasResponse.json();
    const personSchema = schemasData.types?.find((s: any) => s.name === "Person");

    if (!personSchema || !personSchema.ouId) {
      throw new Error("Person user type not found or missing organization unit");
    }

    const defaultOuId = personSchema.ouId;

    // Create user
    const response = await this.request.post(`${this.config.serverUrl}/users`, {
      data: {
        type: "Person",
        ouId: defaultOuId,
        attributes: {
          username: this.config.testUser.username,
          password: this.config.testUser.password,
          given_name: this.config.testUser.given_name,
          email: this.config.testUser.email,
          mobileNumber: this.config.testUser.mobileNumber,
        },
      },
      headers: {
        Authorization: `Bearer ${adminToken}`,
        "Content-Type": "application/json",
      },
      ignoreHTTPSErrors: true,
    });

    if (response.ok()) {
      const data = await response.json();
      return `created:${data.id}`;
    }

    const errorText = await response.text();
    // User might already exist, try to get existing user
    if (response.status() === 409 || errorText.includes("already exists")) {
      const existingId = await this.getExistingUser(adminToken);
      return existingId; // Return without "created:" prefix
    }

    throw new Error(`Failed to create test user: ${errorText}`);
  }

  /**
   * Get existing user by username
   */
  private async getExistingUser(adminToken: string): Promise<string> {
    const response = await this.request.get(
      `${this.config.serverUrl}/users?filter=username eq "${this.config.testUser.username}"`,
      {
        headers: {
          Authorization: `Bearer ${adminToken}`,
        },
        ignoreHTTPSErrors: true,
      }
    );

    if (!response.ok()) {
      throw new Error(`Failed to fetch existing user: ${await response.text()}`);
    }

    const data = await response.json();
    if (!data.users || data.users.length === 0) {
      throw new Error("User exists but could not be found");
    }

    return data.users[0].id;
  }

  /**
   * Update application with MFA authentication and registration flows
   */
  private async updateApplicationFlows(
    adminToken: string,
    authFlowId: string,
    registrationFlowId: string
  ): Promise<string> {
    // First, get all applications and find the one with clientId = "REACT_SDK_SAMPLE"
    const listResponse = await this.request.get(`${this.config.serverUrl}/applications`, {
      headers: {
        Authorization: `Bearer ${adminToken}`,
      },
      ignoreHTTPSErrors: true,
    });

    if (!listResponse.ok()) {
      throw new Error(`Failed to fetch applications: ${await listResponse.text()}`);
    }

    const listData = await listResponse.json();
    const targetApp = listData.applications?.find((app: any) => app.clientId === "REACT_SDK_SAMPLE");

    if (!targetApp) {
      throw new Error(`Application with clientId "REACT_SDK_SAMPLE" not found`);
    }

    const actualAppId = targetApp.id;

    // Get current application details
    const getResponse = await this.request.get(`${this.config.serverUrl}/applications/${actualAppId}`, {
      headers: {
        Authorization: `Bearer ${adminToken}`,
      },
      ignoreHTTPSErrors: true,
    });

    if (!getResponse.ok()) {
      throw new Error(`Failed to fetch application: ${await getResponse.text()}`);
    }

    const appData = await getResponse.json();

    // Update with new flow IDs
    const updatedApp = {
      ...appData,
      authFlowId: authFlowId,
      registrationFlowId: registrationFlowId,
      isRegistrationFlowEnabled: true,
    };

    const updateResponse = await this.request.put(`${this.config.serverUrl}/applications/${actualAppId}`, {
      data: updatedApp,
      headers: {
        Authorization: `Bearer ${adminToken}`,
        "Content-Type": "application/json",
      },
      ignoreHTTPSErrors: true,
    });

    if (!updateResponse.ok()) {
      throw new Error(`Failed to update application: ${await updateResponse.text()}`);
    }

    return actualAppId;
  }

  /**
   * Delete notification sender
   */
  private async deleteNotificationSender(adminToken: string, senderId: string): Promise<void> {
    let requestContext: APIRequestContext | null = null;
    try {
      // Create a new request context for cleanup
      requestContext = await playwrightRequest.newContext({
        ignoreHTTPSErrors: true,
      });

      const response = await requestContext.delete(
        `${this.config.serverUrl}/notification-senders/message/${senderId}`,
        {
          headers: {
            Authorization: `Bearer ${adminToken}`,
          },
        }
      );

      if (response.ok()) {
        console.log(`✓ Notification sender deleted: ${senderId}`);
      } else {
        console.log(`⚠️  Could not delete notification sender: ${await response.text()}`);
      }
    } catch (error) {
      console.log(`⚠️  Error deleting notification sender: ${error}`);
    } finally {
      if (requestContext) {
        await requestContext.dispose();
      }
    }
  }

  /**
   * Delete flow
   */
  private async deleteFlow(adminToken: string, flowId: string): Promise<void> {
    let requestContext: APIRequestContext | null = null;
    try {
      // Create a new request context for cleanup
      requestContext = await playwrightRequest.newContext({
        ignoreHTTPSErrors: true,
      });

      const response = await requestContext.delete(`${this.config.serverUrl}/flows/${flowId}`, {
        headers: {
          Authorization: `Bearer ${adminToken}`,
        },
      });

      if (response.ok()) {
        console.log(`✓ Flow deleted: ${flowId}`);
      } else {
        console.log(`⚠️  Could not delete flow: ${await response.text()}`);
      }
    } catch (error) {
      console.log(`⚠️  Error deleting flow: ${error}`);
    } finally {
      if (requestContext) {
        await requestContext.dispose();
      }
    }
  }

  /**
   * Delete user
   */
  private async deleteUser(adminToken: string, userId: string): Promise<void> {
    let requestContext: APIRequestContext | null = null;
    try {
      // Create a new request context for cleanup
      requestContext = await playwrightRequest.newContext({
        ignoreHTTPSErrors: true,
      });

      const response = await requestContext.delete(`${this.config.serverUrl}/users/${userId}`, {
        headers: {
          Authorization: `Bearer ${adminToken}`,
        },
      });

      if (response.ok()) {
        console.log(`✓ User deleted: ${userId}`);
      } else {
        console.log(`⚠️  Could not delete user: ${await response.text()}`);
      }
    } catch (error) {
      console.log(`⚠️  Error deleting user: ${error}`);
    } finally {
      if (requestContext) {
        await requestContext.dispose();
      }
    }
  }

  /**
   * Get MFA flow node definitions with senderId injected
   */
  private getMFAFlowNodes(senderId: string): any[] {
    // Deep clone the template and replace senderId placeholder
    const nodesJson = JSON.stringify(mfaFlowNodesTemplate);
    const nodesWithSenderId = nodesJson.replace(/\{\{SENDER_ID\}\}/g, senderId);
    return JSON.parse(nodesWithSenderId);
  }

  /**
   * Get MFA registration flow node definitions
   */
  private getMFARegistrationFlowNodes(): any[] {
    return mfaRegistrationFlowNodesTemplate;
  }
}
