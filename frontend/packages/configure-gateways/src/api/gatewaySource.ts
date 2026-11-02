// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMemo} from 'react';
import useRequest from './request';
import type {DeploySource, StoredValueInput} from './source';
import type {
  AppliedConfiguration,
  AppliedVersion,
  ApplyRequest,
  ApplyResult,
  StoredValue,
  ValueKind,
  ValueReference,
  Version,
} from '../models/deploy';
import type {Gateway} from '../models/gateway';

/** How many values a gateway's store answers with in one page. */
const PAGE_SIZE = 100;

const gateway = (id: string): string => `/gateways/${encodeURIComponent(id)}`;
const collectionOf = (kind: ValueKind): string => (kind === 'secret' ? 'secrets' : 'variables');

/** The control plane's own source: its gateways and the configuration versions it captures. */
export default function useGatewaySource(): DeploySource {
  const request = useRequest();
  return useMemo((): DeploySource => {
    const version = (ref: string): Promise<Version> =>
      request<Version>('GET', `/configuration-versions/${encodeURIComponent(ref)}`);
    const appliedVersion = (targetId: string): Promise<AppliedVersion> =>
      request<AppliedVersion>('GET', `${gateway(targetId)}/applied-version`);
    return {
      targets: async (): Promise<Gateway[]> => {
        const answer = await request<Gateway[] | {gateways?: Gateway[]}>('GET', '/gateways');
        const gateways = Array.isArray(answer) ? answer : (answer.gateways ?? []);
        return [...gateways].sort(
          (a: Gateway, b: Gateway) => Number(b.isDefault ?? false) - Number(a.isDefault ?? false),
        );
      },
      versions: async (): Promise<Version[]> => {
        const versions = await request<Version[] | {versions?: Version[]}>('GET', '/configuration-versions');
        return Array.isArray(versions) ? versions : (versions.versions ?? []);
      },
      version,
      appliedVersion,
      appliedConfiguration: (targetId: string): Promise<AppliedConfiguration> =>
        request<AppliedConfiguration>('GET', `${gateway(targetId)}/applied-configuration`),
      applyCheck: (targetId: string, ref: string): Promise<ApplyResult> =>
        request<ApplyResult>('POST', `${gateway(targetId)}/apply`, {
          version: ref,
          dryRun: true,
        } satisfies ApplyRequest),
      apply: (targetId: string, data: ApplyRequest): Promise<ApplyResult> =>
        request<ApplyResult>('POST', `${gateway(targetId)}/apply`, data),
      revert: (targetId: string, dryRun: boolean, confirmRemovals?: boolean): Promise<ApplyResult> =>
        request<ApplyResult>('POST', `${gateway(targetId)}/revert`, {
          dryRun,
          ...(confirmRemovals ? {confirmRemovals} : {}),
        }),
      values: async (targetId: string, kind: ValueKind): Promise<StoredValue[]> => {
        const collection = collectionOf(kind);
        const values: StoredValue[] = [];
        for (;;) {
          const page = await request<Record<string, unknown>>(
            'GET',
            `${gateway(targetId)}/${collection}?limit=${PAGE_SIZE}&offset=${values.length}`,
          );
          const entries = (page[collection] as StoredValue[] | undefined) ?? [];
          values.push(...entries);
          if (entries.length === 0 || values.length >= Number(page['totalResults'] ?? values.length)) {
            return values;
          }
        }
      },
      setValue: (targetId: string, kind: ValueKind, name: string, input: StoredValueInput): Promise<StoredValue> =>
        request<StoredValue>('PUT', `${gateway(targetId)}/${collectionOf(kind)}/${encodeURIComponent(name)}`, {
          value: input.value,
          description: input.description,
        }),
      deleteValue: (targetId: string, kind: ValueKind, name: string): Promise<void> =>
        request<void>('DELETE', `${gateway(targetId)}/${collectionOf(kind)}/${encodeURIComponent(name)}`),
      runningReferences: async (targetId: string): Promise<ValueReference[]> => {
        const applied = await appliedVersion(targetId);
        if (!applied.appliedVersion) return [];
        return (await version(applied.appliedVersion)).references ?? [];
      },
    };
  }, [request]);
}
