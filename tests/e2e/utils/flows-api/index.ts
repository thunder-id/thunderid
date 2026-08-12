// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Flows API Helper
 *
 * Thin wrapper over the backend `/flows` endpoint for E2E setup and teardown (MFA and social
 * login both create an authentication flow and need to find/reuse one left over from an
 * earlier run).
 *
 * Constructible directly (`new FlowsApi(request)`) so `beforeAll`/`afterAll` can use it -
 * Playwright forbids custom test-scoped fixtures in those hooks.
 */

import type { APIRequestContext } from "@playwright/test";
import { send, sendOk } from "../api-request";

export type ApiFlow = {
  id: string;
  handle: string;
  name: string;
  flowType: string;
  nodes?: unknown[];
};

// Largest page the backend allows (`maxPageSize` in backend/internal/flow/mgt/constants.go),
// used to minimize the number of pages findByHandle has to walk.
const maxPageSize = 100;

export class FlowsApi {
  constructor(private readonly request: APIRequestContext) {}

  /** Create a flow directly via the API. */
  async create(data: Record<string, unknown>): Promise<ApiFlow> {
    const response = await sendOk(this.request, "POST", "/flows", data);
    return (await response.json()) as ApiFlow;
  }

  /**
   * Look up a flow by its handle, optionally narrowed by flowType. `GET /flows` only supports
   * `flowType`, `limit` and `offset` (backend/internal/flow/mgt/handler.go) - there is no
   * handle filter - so this walks every page (narrowed server-side by flowType when given) and
   * matches the handle client-side.
   */
  async findByHandle(handle: string, flowType?: string): Promise<ApiFlow | undefined> {
    const typeQuery = flowType ? `&flowType=${encodeURIComponent(flowType)}` : "";

    for (let offset = 0; ; ) {
      const response = await sendOk(this.request, "GET", `/flows?limit=${maxPageSize}&offset=${offset}${typeQuery}`);
      const body = (await response.json()) as { flows?: ApiFlow[]; totalResults?: number };
      const match = body.flows?.find(f => f.handle === handle);
      if (match) {
        return match;
      }
      if (!body.flows?.length || offset + body.flows.length >= (body.totalResults ?? 0)) {
        return undefined;
      }
      offset += body.flows.length;
    }
  }

  /** Overwrite a flow's fields. Used to re-point a reused flow's nodes at fresh dependency ids. */
  async update(id: string, data: Record<string, unknown>): Promise<void> {
    await sendOk(this.request, "PUT", `/flows/${id}`, data);
  }

  /** Delete by id. A 404 counts as success so retries and double-teardown stay idempotent. */
  async deleteById(id: string): Promise<boolean> {
    const response = await send(this.request, "DELETE", `/flows/${id}`);
    return response.ok() || response.status() === 404;
  }
}
