// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Applications API Helper
 *
 * Thin wrapper over the backend `/applications` endpoint for E2E setup and teardown.
 *
 * Used two ways, with no duplicated logic:
 * - directly (`new ApplicationsApi(request)`) from `beforeAll`/`afterAll`, because Playwright
 *   forbids custom test-scoped fixtures in those hooks;
 * - via the `applicationsApi` fixture inside test bodies.
 */

import type { APIRequestContext } from "@playwright/test";
import { send, sendOk } from "../api-request";

export type ApiApplication = {
  id: string;
  name: string;
  clientId?: string;
  authFlowId?: string;
  registrationFlowId?: string;
  recoveryFlowId?: string | null;
  isRegistrationFlowEnabled?: boolean;
  allowedUserTypes?: string[];
  [key: string]: unknown;
};

export class ApplicationsApi {
  constructor(private readonly request: APIRequestContext) {}

  /** Create an application directly via the API. */
  async create(data: Record<string, unknown>): Promise<ApiApplication> {
    const response = await sendOk(this.request, "POST", "/applications", data);
    return (await response.json()) as ApiApplication;
  }

  /** Read a single application by id. */
  async get(id: string): Promise<ApiApplication> {
    const response = await sendOk(this.request, "GET", `/applications/${id}`);
    return (await response.json()) as ApiApplication;
  }

  /**
   * Look up an application by its OAuth2 clientId.
   *
   * The list endpoint has no filter parameter, so the match is client-side.
   */
  async findByClientId(clientId: string): Promise<ApiApplication | undefined> {
    const response = await sendOk(this.request, "GET", "/applications");
    const body = (await response.json()) as { applications?: ApiApplication[] };
    return body.applications?.find((app) => app.clientId === clientId);
  }

  /** Overwrite an application's fields. Callers spread the current record first. */
  async update(id: string, data: Record<string, unknown>): Promise<void> {
    await sendOk(this.request, "PUT", `/applications/${id}`, data);
  }

  /** Delete by id. A 404 counts as success so retries and double-teardown stay idempotent. */
  async deleteById(id: string): Promise<boolean> {
    const response = await send(this.request, "DELETE", `/applications/${id}`);
    return response.ok() || response.status() === 404;
  }
}
