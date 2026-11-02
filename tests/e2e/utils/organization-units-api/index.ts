// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Organization Units API Helper
 *
 * Thin wrapper over the backend `/organization-units` endpoint for E2E setup and teardown.
 * Used directly (`new OrganizationUnitsApi(request)`) from hooks, and via the
 * `organizationUnitsApi` fixture inside test bodies, same as `UsersApi`.
 */

import type { APIRequestContext } from "@playwright/test";
import { send, sendOk } from "../api-request";

export type ApiOrganizationUnit = {
  id: string;
  handle: string;
  name: string;
  parent?: string | null;
};

export class OrganizationUnitsApi {
  constructor(private readonly request: APIRequestContext) {}

  /** Create a unit under `parent`, or a root unit when `parent` is null. */
  async create(handle: string, name: string, parent: string | null): Promise<ApiOrganizationUnit> {
    const response = await sendOk(this.request, "POST", "/organization-units", { handle, name, parent });
    return (await response.json()) as ApiOrganizationUnit;
  }

  /**
   * Delete by id. A 404 counts as success so retries and double-teardown stay idempotent.
   *
   * Never throws: this is only ever called from test teardown. A unit that still has child units
   * or users is refused, so delete those first.
   */
  async deleteById(id: string): Promise<boolean> {
    try {
      const response = await send(this.request, "DELETE", `/organization-units/${id}`);
      return response.ok() || response.status() === 404;
    } catch (error) {
      console.warn(`Cleanup skipped for organization unit ${id}: ${String(error)}`);
      return false;
    }
  }
}
