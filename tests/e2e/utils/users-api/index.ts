// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Users API Helper
 *
 * Thin wrapper over the backend `/users` endpoint for E2E verification and teardown.
 *
 * Used two ways, with no duplicated logic:
 * - directly (`new UsersApi(request)`) from `beforeAll`/`afterAll`, because Playwright
 *   forbids custom test-scoped fixtures in those hooks;
 * - via the `usersApi` fixture inside test bodies.
 */

import type { APIRequestContext } from "@playwright/test";
import { send, sendOk, type ApiAccessStatus } from "../api-request";
import { UserTypesApi } from "../user-types-api";

export type ApiUser = {
  id: string;
  ouId: string;
  type: string;
  attributes: Record<string, unknown> & { username?: string; email?: string };
  status?: ApiAccessStatus;
};

/** A user sharing policy, as `/users/{id}/sharing-policies` returns it. */
export type ApiSharingPolicy = {
  id: string;
  initiatingOuId: string;
  stage: "share" | "reshare";
  version: number;
  childUnits?:
    | { mode: "all"; excludedOuIds?: string[] }
    | { mode: "selected"; units: { ouId: string; reach: string }[] };
  otherTreeRoots?: { mode: "all"; excludedOuIds?: string[] } | { mode: "selected"; units: { ouId: string }[] };
};

export class UsersApi {
  private readonly userTypes: UserTypesApi;

  constructor(private readonly request: APIRequestContext) {
    this.userTypes = new UserTypesApi(request);
  }

  /** Create a user directly via the API, bypassing the Console's Create User wizard. */
  async createUser(attributes: Record<string, unknown>, type: string = "person"): Promise<ApiUser> {
    const userType = await this.userTypes.findByHandle(type);
    if (!userType) {
      throw new Error(`GET /user-types returned no "${type}" user type`);
    }
    const response = await sendOk(this.request, "POST", "/users", {
      ouId: userType.ouId,
      // CreateUser resolves `type` by entity-type handle (e.g. "person"), not by id.
      type: userType.handle,
      attributes,
    });
    return (await response.json()) as ApiUser;
  }

  /**
   * Look up users by a single `attribute eq "value"` clause.
   *
   * That is the whole filter grammar the backend accepts - one clause, `eq` only, no
   * and/or - and the attribute path resolves inside the user's `attributes` JSON.
   */
  async findBy(attribute: string, value: string): Promise<ApiUser[]> {
    if (value.includes('"')) {
      throw new Error(`findBy value must not contain a double quote (unsupported by the filter grammar): ${value}`);
    }
    const filter = encodeURIComponent(`${attribute} eq "${value}"`);
    const response = await sendOk(this.request, "GET", `/users?filter=${filter}`);
    const body = (await response.json()) as { users?: ApiUser[] };
    return body.users ?? [];
  }

  /** Read a single user by id, including its access status. */
  async get(id: string): Promise<ApiUser> {
    return (await sendOk(this.request, "GET", `/users/${id}`)).json() as Promise<ApiUser>;
  }

  async findByUsername(username: string): Promise<ApiUser | undefined> {
    return (await this.findBy("username", username))[0];
  }

  /** Record a sharing policy for the user, issued by its own unit unless the body names `initiatingOuId`. */
  async share(userId: string, policy: Record<string, unknown>): Promise<ApiSharingPolicy> {
    const response = await sendOk(this.request, "POST", `/users/${userId}/sharing-policies`, policy);
    return (await response.json()) as ApiSharingPolicy;
  }

  /** Every sharing policy of the user, re-shares included. */
  async listSharingPolicies(userId: string): Promise<ApiSharingPolicy[]> {
    const response = await sendOk(this.request, "GET", `/users/${userId}/sharing-policies?limit=100`);
    const body = (await response.json()) as { policies?: ApiSharingPolicy[] };
    return body.policies ?? [];
  }

  /**
   * Delete by id. A 404 counts as success so retries and double-teardown stay idempotent.
   *
   * Never throws: this is only ever called from test teardown, and a flaky network error there
   * must not mask the failure (or success) a test already reported.
   */
  async deleteById(id: string): Promise<boolean> {
    try {
      const response = await send(this.request, "DELETE", `/users/${id}`);
      return response.ok() || response.status() === 404;
    } catch (error) {
      console.warn(`Cleanup skipped for user ${id}: ${String(error)}`);
      return false;
    }
  }

  /**
   * Resolve a username to its id(s) and delete them. Returns how many were deleted.
   *
   * Never throws: teardown must not mask the failure that a test already reported.
   * Never sweeps: the filter is an exact `eq` on a per-run-unique username, so concurrent
   * chromium/firefox/webkit runs cannot delete each other's users.
   */
  async deleteByUsername(username: string): Promise<number> {
    try {
      const users = await this.findBy("username", username);
      let deleted = 0;
      for (const user of users) {
        if (await this.deleteById(user.id)) {
          deleted += 1;
        } else {
          console.warn(`Failed to delete test user ${user.id} (${username})`);
        }
      }
      return deleted;
    } catch (error) {
      console.warn(`Cleanup skipped for ${username}: ${String(error)}`);
      return 0;
    }
  }
}
