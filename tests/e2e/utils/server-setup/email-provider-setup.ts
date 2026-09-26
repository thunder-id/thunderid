// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Email Provider Setup
 *
 * Email delivery is configured only through `/connections/email-smtp`: there is no deployment-wide
 * default a flow can fall back to, so every EmailExecutor node names the provider it sends
 * through in its `senderId` property.
 *
 * The distribution ships flows whose email steps carry no `senderId` (no provider exists before
 * the server is up), so a spec that reads mail from the mock inbox has to create a provider and
 * stamp its id onto those steps first. `setUpMockEmailProvider` does both and returns a teardown
 * that restores every flow it touched and deletes the provider.
 *
 * The provider is a shared, name-addressed resource and the flows it patches are shared too, so
 * nothing may run this concurrently with itself - the calling spec is serial and runs in its own
 * Playwright project.
 */

import type { APIRequestContext } from "@playwright/test";
import { ConnectionsApi } from "../connections-api";
import { FlowsApi, type ApiFlow } from "../flows-api";

const SMTP_VENDOR = "email-smtp";
const PROVIDER_NAME = "E2E Mock SMTP Provider";
const EMAIL_EXECUTOR = "EmailExecutor";

/** Matches the mock SMTP server defaults in run-e2e.sh and pr-builder.yml. */
const MOCK_SMTP_HOST = "127.0.0.1";
const MOCK_SMTP_PORT = 2525;
const MOCK_SMTP_FROM_ADDRESS = "no-reply@thunderid.test";

type FlowNode = { properties?: Record<string, unknown>; executor?: { name?: string } };

export type MockEmailProviderSetup = {
  providerId: string;
  /** Restore the patched flows and delete the provider. Never throws. */
  tearDown: () => Promise<void>;
};

/** Reuse a provider left over from an earlier run, since its name is fixed. */
async function ensureProvider(connectionsApi: ConnectionsApi): Promise<{ id: string; created: boolean }> {
  const existing = await connectionsApi.findByName(SMTP_VENDOR, PROVIDER_NAME);
  if (existing) {
    return { id: existing.id, created: false };
  }

  const created = await connectionsApi.create(SMTP_VENDOR, {
    name: PROVIDER_NAME,
    description: "Points the E2E suite's email steps at the mock SMTP server",
    host: MOCK_SMTP_HOST,
    port: MOCK_SMTP_PORT,
    fromAddress: MOCK_SMTP_FROM_ADDRESS,
    tls: "none",
    authentication: { type: "none" },
  });

  return { id: created.id, created: true };
}

/** The email steps of `flow` that do not already name a provider. */
function unassignedEmailNodes(flow: ApiFlow): FlowNode[] {
  return ((flow.nodes ?? []) as FlowNode[]).filter(
    node => node.executor?.name === EMAIL_EXECUTOR && !node.properties?.senderId
  );
}

/**
 * Create the mock email provider and select it on every email step of the named flows that does
 * not already name one. A handle that resolves to no flow is skipped rather than failing: the
 * caller's own assertions cover whether the flow it needs exists.
 */
export async function setUpMockEmailProvider(
  request: APIRequestContext,
  flowHandles: readonly string[]
): Promise<MockEmailProviderSetup> {
  const connectionsApi = new ConnectionsApi(request);
  const flowsApi = new FlowsApi(request);

  const provider = await ensureProvider(connectionsApi);
  const patchedFlows: ApiFlow[] = [];

  for (const handle of flowHandles) {
    const summary = await flowsApi.findByHandle(handle);
    if (!summary) {
      continue;
    }

    const flow = await flowsApi.get(summary.id);
    const nodes = unassignedEmailNodes(flow);
    if (nodes.length === 0) {
      continue;
    }

    // Keep the flow as read so teardown can put the email steps back the way they were.
    patchedFlows.push(JSON.parse(JSON.stringify(flow)) as ApiFlow);

    for (const node of nodes) {
      node.properties = { ...(node.properties ?? {}), senderId: provider.id };
    }
    await flowsApi.update(flow.id, flow as unknown as Record<string, unknown>);
  }

  return {
    providerId: provider.id,
    tearDown: async () => {
      for (const flow of patchedFlows) {
        try {
          await flowsApi.update(flow.id, flow as unknown as Record<string, unknown>);
        } catch (error) {
          console.warn(`Teardown skipped for flow ${flow.id}: ${String(error)}`);
        }
      }

      // Only remove a provider this run created: a leftover one may belong to a parallel project.
      if (provider.created) {
        await connectionsApi.deleteById(SMTP_VENDOR, provider.id);
      }
    },
  };
}
