// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Agents API Helper
 *
 * Thin wrapper over the backend `/agents` endpoint. The onboarding spec creates agents through
 * the Console flow and needs to both verify and remove them out of band.
 *
 * Constructible directly (`new AgentsApi(request)`) so `beforeAll`/`afterAll` can use it -
 * Playwright forbids custom test-scoped fixtures in those hooks.
 */

import type { APIRequestContext } from "@playwright/test";
import { send, sendOk } from "../api-request";

/** An agent as returned by `/agents`. */
export type ApiAgent = {
  id: string;
  name?: string;
  clientId?: string;
  owner?: string;
  type?: string;
  ouId?: string;
  attributes?: Record<string, unknown>;
};

export class AgentsApi {
  constructor(private readonly request: APIRequestContext) {}

  /**
   * Every agent in the system. The list endpoint has no name filter and `limit` caps at 100,
   * so this pages until every page has been read.
   */
  async list(): Promise<ApiAgent[]> {
    const pageSize = 100;
    const all: ApiAgent[] = [];

    for (let offset = 0; ; offset += pageSize) {
      const response = await sendOk(this.request, "GET", `/agents?limit=${pageSize}&offset=${offset}`);
      const body = (await response.json()) as { agents?: ApiAgent[]; totalResults?: number };
      const agents = body.agents ?? [];
      all.push(...agents);

      if (agents.length === 0 || all.length >= (body.totalResults ?? 0)) return all;
    }
  }

  /** Resolve an agent by name, the way a test names the one it just created. */
  async findByName(name: string): Promise<ApiAgent | undefined> {
    return (await this.list()).find(agent => agent.name === name);
  }

  /**
   * Read a single agent by id. Unlike the list endpoint this returns the full record, including
   * the attributes the onboarding flow collected.
   */
  async get(id: string): Promise<ApiAgent> {
    return (await sendOk(this.request, "GET", `/agents/${id}`)).json() as Promise<ApiAgent>;
  }

  /** Delete by id. The backend returns 204 for an unknown id too, so this is idempotent. */
  async deleteById(id: string): Promise<boolean> {
    return (await send(this.request, "DELETE", `/agents/${id}`)).ok();
  }

  /**
   * Resolve a name to its agent and delete it. Returns whether anything was removed.
   *
   * Never throws: teardown must not mask the failure that a test already reported.
   */
  async deleteByName(name: string): Promise<boolean> {
    try {
      const agent = await this.findByName(name);
      if (!agent) return false;
      if (await this.deleteById(agent.id)) return true;
      console.warn(`Failed to delete test agent ${agent.id} (${name})`);
      return false;
    } catch (error) {
      console.warn(`Cleanup skipped for agent ${name}: ${String(error)}`);
      return false;
    }
  }
}
