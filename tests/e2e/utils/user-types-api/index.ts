// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * User Types API Helper
 *
 * Thin wrapper over the backend `/user-types` endpoint. Owns the "resolve a user type by
 * name" lookup that the users API, the MFA server setup, and the sample-app specs all need.
 *
 * Constructible directly (`new UserTypesApi(request)`) so `beforeAll`/`afterAll` can use it -
 * Playwright forbids custom test-scoped fixtures in those hooks.
 */

import type { APIRequestContext } from "@playwright/test";
import { send, sendOk } from "../api-request";

/** A user type as returned by `/user-types`. `schema` is only present on single-type reads. */
export type ApiUserType = {
  id: string;
  name: string;
  ouId: string;
  allowSelfRegistration?: boolean;
  systemAttributes?: { display?: string };
  schema?: Record<string, Record<string, unknown>>;
};

export class UserTypesApi {
  constructor(private readonly request: APIRequestContext) {}

  /**
   * Every user type in the system. The list endpoint has no filter parameter and `limit` caps
   * at 100, so this pages until every page has been read.
   */
  async list(): Promise<ApiUserType[]> {
    const pageSize = 100;
    const all: ApiUserType[] = [];

    for (let offset = 0; ; offset += pageSize) {
      const response = await sendOk(this.request, "GET", `/user-types?limit=${pageSize}&offset=${offset}`);
      const body = (await response.json()) as { types?: ApiUserType[]; totalResults?: number };
      const types = body.types ?? [];
      all.push(...types);

      if (types.length === 0 || all.length >= (body.totalResults ?? 0)) return all;
    }
  }

  /**
   * Resolve a user type by name, the same way the Console's wizards do, rather than
   * hardcoding the bootstrap-default ids.
   */
  async findByName(name: string): Promise<ApiUserType | undefined> {
    return (await this.list()).find(type => type.name === name);
  }

  /** Read a single user type by id. Unlike the list endpoint, this includes `schema`. */
  async get(id: string): Promise<ApiUserType> {
    const response = await sendOk(this.request, "GET", `/user-types/${id}`);
    return (await response.json()) as ApiUserType;
  }

  /** Delete by id. The backend returns 204 for an unknown id too, so this is idempotent. */
  async deleteById(id: string): Promise<boolean> {
    return (await send(this.request, "DELETE", `/user-types/${id}`)).ok();
  }

  /**
   * Resolve a name to its user type and delete it. Returns whether anything was removed.
   *
   * Never throws: teardown must not mask the failure that a test already reported. Delete the
   * type's users first - `ENTITY.TYPE` holds the type name, not a foreign key, so removing a
   * type with live users orphans them silently.
   */
  async deleteByName(name: string): Promise<boolean> {
    try {
      const userType = await this.findByName(name);
      if (!userType) return false;
      if (await this.deleteById(userType.id)) return true;
      console.warn(`Failed to delete test user type ${userType.id} (${name})`);
      return false;
    } catch (error) {
      console.warn(`Cleanup skipped for user type ${name}: ${String(error)}`);
      return false;
    }
  }
}
