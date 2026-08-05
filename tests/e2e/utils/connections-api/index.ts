// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Connections API Helper
 *
 * Thin wrapper over the backend `/connections/{vendor}` endpoints (identity providers like
 * "google"/"github" and notification senders like "sms-gateway" share this URL shape and the
 * same list/get/delete response shaping, even though they are different resource types
 * server-side - see backend/internal/connection/handler.go listInstances/listSMSInstances,
 * both of which write a bare array of {id, name, description} for the list route).
 *
 * Constructible directly (`new ConnectionsApi(request)`) so `beforeAll`/`afterAll` can use it -
 * Playwright forbids custom test-scoped fixtures in those hooks.
 */

import type { APIRequestContext } from "@playwright/test";
import { send, sendOk } from "../api-request";

export type ApiConnection = {
  id: string;
  name: string;
  description?: string;
};

/** Single-resource detail, which (unlike the list endpoint) includes vendor-specific fields. */
export type ApiConnectionDetail = ApiConnection & {
  url?: string;
  httpMethod?: string;
  contentType?: string;
};

export class ConnectionsApi {
  constructor(private readonly request: APIRequestContext) {}

  /** List every connection of a vendor. */
  async list(vendor: string): Promise<ApiConnection[]> {
    const response = await sendOk(this.request, "GET", `/connections/${vendor}`);
    return (await response.json()) as ApiConnection[];
  }

  /** Create a connection directly via the API. */
  async create(vendor: string, data: Record<string, unknown>): Promise<ApiConnection> {
    const response = await sendOk(this.request, "POST", `/connections/${vendor}`, data);
    return (await response.json()) as ApiConnection;
  }

  /** Look up a connection of a vendor by name. The list endpoint has no filter parameter. */
  async findByName(vendor: string, name: string): Promise<ApiConnection | undefined> {
    return (await this.list(vendor)).find(c => c.name === name);
  }

  /** Read a single connection's full detail (includes vendor-specific fields like `url`). */
  async get(vendor: string, id: string): Promise<ApiConnectionDetail> {
    const response = await sendOk(this.request, "GET", `/connections/${vendor}/${id}`);
    return (await response.json()) as ApiConnectionDetail;
  }

  /** Overwrite a connection's fields (full replace, matching the backend's PUT semantics). */
  async update(vendor: string, id: string, data: Record<string, unknown>): Promise<ApiConnectionDetail> {
    const response = await sendOk(this.request, "PUT", `/connections/${vendor}/${id}`, data);
    return (await response.json()) as ApiConnectionDetail;
  }

  /** Delete by id. A 404 counts as success so retries and double-teardown stay idempotent. */
  async deleteById(vendor: string, id: string): Promise<boolean> {
    const response = await send(this.request, "DELETE", `/connections/${vendor}/${id}`);
    return response.ok() || response.status() === 404;
  }
}
