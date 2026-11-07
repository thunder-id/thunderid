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
  Master,
  ResourceHistory,
  StoredValue,
  ValueKind,
  ValueReference,
  Version,
} from '../models/deploy';
import type {Gateway} from '../models/gateway';
import type {AutoApplySetting, Pipeline} from '../models/pipeline';
import type {Project} from '../models/project';
import type {ProjectStatus} from '../models/projectStatus';

/** How many values a gateway's store answers with in one page. */
const PAGE_SIZE = 100;

const gateway = (id: string): string => `/gateways/${encodeURIComponent(id)}`;
const collectionOf = (kind: ValueKind): string => (kind === 'secret' ? 'secrets' : 'variables');

/** The server configuration section that says how this control plane deploys. */
const SETTINGS_SECTION = '/server-config/gateway';

/** The gateway section: the default for projects, and a project's own choice where it makes one. */
interface GatewaySettings {
  autoApply?: boolean;
  projects?: Record<string, unknown>;
}

/** The gateway section's layers, as the server configuration API reads them. */
interface SettingsLayers {
  readOnly?: GatewaySettings | null;
  writable?: GatewaySettings | null;
  merged?: GatewaySettings | null;
}

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
      master: (targetId: string): Promise<Master> => request<Master>('GET', `${gateway(targetId)}/master`),
      masterHistory: (targetId: string): Promise<Master[]> =>
        request<Master[]>('GET', `${gateway(targetId)}/master/history`),
      resourceHistory: (targetId: string, key: string): Promise<ResourceHistory> =>
        request<ResourceHistory>(
          'GET',
          `${gateway(targetId)}/master/resources/${key.split('/').map(encodeURIComponent).join('/')}`,
        ),
      applyCheck: (targetId: string, ref: string, project?: string): Promise<ApplyResult> =>
        request<ApplyResult>('POST', `${gateway(targetId)}/apply`, {
          version: ref,
          dryRun: true,
          ...(project ? {project} : {}),
        } satisfies ApplyRequest),
      apply: (targetId: string, data: ApplyRequest): Promise<ApplyResult> =>
        request<ApplyResult>('POST', `${gateway(targetId)}/apply`, data),
      revert: (targetId: string, dryRun: boolean, confirmRemovals?: boolean, project?: string): Promise<ApplyResult> =>
        request<ApplyResult>('POST', `${gateway(targetId)}/revert`, {
          dryRun,
          ...(confirmRemovals ? {confirmRemovals} : {}),
          ...(project ? {project} : {}),
        }),
      projectStatus: (targetId: string, project: string, from?: string): Promise<ProjectStatus> => {
        const query = new URLSearchParams({project, ...(from ? {from} : {})});
        return request<ProjectStatus>('GET', `${gateway(targetId)}/project-status?${query.toString()}`);
      },
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
      // Every project is deployed through the one pipeline the control plane is configured with.
      pipeline: async (): Promise<Pipeline> => {
        const answer = await request<Pipeline>('GET', '/gateways/pipeline');
        return {stages: answer.stages ?? []};
      },
      projects: async (): Promise<Project[]> => {
        const answer = await request<{projects?: Project[]}>('GET', '/projects');
        return (answer.projects ?? []).map(({id, displayName}: Project) => ({id, displayName}));
      },
      autoApply: async (): Promise<AutoApplySetting> => {
        const layers = await request<SettingsLayers>('GET', SETTINGS_SECTION);
        return {
          enabled: layers.merged?.autoApply ?? false,
          locked: layers.readOnly?.autoApply !== undefined && layers.readOnly?.autoApply !== null,
        };
      },
      setAutoApply: async (enabled: boolean): Promise<void> => {
        const layers = await request<SettingsLayers>('GET', SETTINGS_SECTION);
        await request<unknown>('PUT', SETTINGS_SECTION, {...(layers.writable ?? {}), autoApply: enabled});
      },
      // Only this project's choice is written; null has it follow the organization's default again.
      setProjectAutoSync: async (project: string, enabled: boolean | null): Promise<void> => {
        await request<unknown>('PUT', `/projects/${encodeURIComponent(project)}/auto-sync`, {
          autoSync: enabled,
        });
      },
    };
  }, [request]);
}
