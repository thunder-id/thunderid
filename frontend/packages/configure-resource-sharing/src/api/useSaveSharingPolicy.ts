// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import ResourceSharingQueryKeys from '../constants/resource-sharing-query-keys';
import type {SharingPolicySavePlan} from '../models/organization-access';
import type {SharingPolicyRequest} from '../models/sharing-policy';

/**
 * Carries out a sharing policy save plan against one shareable resource.
 *
 * A replace is a delete followed by a create, because the resource may hold only one owner-issued
 * policy. The two are not atomic: if the create fails, the resource is left unshared, the query is
 * refreshed so the screen shows that, and saving again creates the policy.
 *
 * Failures are left to the caller, which shows them beside the save action.
 *
 * @param resourcePath - The resource's path relative to the server URL, e.g. `applications/<id>`
 * @returns TanStack Query mutation that takes a {@link SharingPolicySavePlan}
 *
 * @public
 */
export default function useSaveSharingPolicy(
  resourcePath: string,
): UseMutationResult<void, Error, SharingPolicySavePlan> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();
  const {t} = useTranslation('resourceSharing');
  const {showToast} = useToast();

  return useMutation<void, Error, SharingPolicySavePlan>({
    mutationFn: async (plan: SharingPolicySavePlan): Promise<void> => {
      const collectionUrl = `${getServerUrl()}/${resourcePath}/sharing-policies`;
      const send = (url: string, method: string, data?: SharingPolicyRequest): Promise<unknown> =>
        http.request({
          url,
          method,
          headers: {'Content-Type': 'application/json'},
          data,
        } as unknown as Parameters<typeof http.request>[0]);

      switch (plan.type) {
        case 'create':
          await send(collectionUrl, 'POST', plan.sections);
          break;
        case 'update':
          await send(`${collectionUrl}/${encodeURIComponent(plan.policyId)}`, 'PUT', {
            ...plan.sections,
            version: plan.version,
          });
          break;
        case 'delete':
          await send(`${collectionUrl}/${encodeURIComponent(plan.policyId)}`, 'DELETE');
          break;
        case 'replace':
          await send(`${collectionUrl}/${encodeURIComponent(plan.policyId)}`, 'DELETE');
          await send(collectionUrl, 'POST', plan.sections);
          break;
        default:
          break;
      }
    },
    onSuccess: () => {
      showToast(t('save.success', 'Organization access updated.'), 'success');
    },
    onSettled: () => {
      queryClient.invalidateQueries({queryKey: [ResourceSharingQueryKeys.SHARING_POLICIES, resourcePath]}).catch(() => {
        // Ignore invalidation errors
      });
    },
  });
}
