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
import { send, sendOk } from "../api-request";
import { UserTypesApi } from "../user-types-api";

export type ApiUser = {
  id: string;
  ouId: string;
  type: string;
  attributes: Record<string, unknown> & { username?: string; email?: string };
};

export class UsersApi {
  private readonly userTypes: UserTypesApi;

  constructor(private readonly request: APIRequestContext) {
    this.userTypes = new UserTypesApi(request);
  }

  /** Create a user directly via the API, bypassing the Console's Create User wizard. */
  async createUser(attributes: Record<string, unknown>, type: string = "Person"): Promise<ApiUser> {
    const userType = await this.userTypes.findByName(type);
    if (!userType) {
      throw new Error(`GET /user-types returned no "${type}" user type`);
    }
    const response = await sendOk(this.request, "POST", "/users", {
      ouId: userType.ouId,
      // CreateUser resolves `type` by entity-type name (e.g. "Person"), not by id.
      type: userType.name,
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

  async findByUsername(username: string): Promise<ApiUser | undefined> {
    return (await this.findBy("username", username))[0];
  }

  /** Delete by id. A 404 counts as success so retries and double-teardown stay idempotent. */
  async deleteById(id: string): Promise<boolean> {
    const response = await send(this.request, "DELETE", `/users/${id}`);
    return response.ok() || response.status() === 404;
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
