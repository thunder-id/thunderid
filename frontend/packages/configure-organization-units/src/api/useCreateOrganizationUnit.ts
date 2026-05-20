/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {useThunderID} from '@thunderid/react';
import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useTranslation} from 'react-i18next';
import OrganizationUnitQueryKeys from '../constants/organization-unit-query-keys';
import type {OrganizationUnit} from '../models/organization-unit';
import type {CreateOrganizationUnitRequest} from '../models/requests';

/**
 * Custom hook to create a new organization unit.
 *
 * @returns TanStack Query mutation object for creating organization units
 *
 * @example
 * ```tsx
 * function CreateOUButton() {
 *   const createOU = useCreateOrganizationUnit();
 *
 *   const handleCreate = (data: CreateOrganizationUnitRequest) => {
 *     createOU.mutate(data, {
 *       onSuccess: (ou) => {
 *         console.log('Organization unit created:', ou);
 *       },
 *       onError: (error) => {
 *         console.error('Failed to create organization unit:', error);
 *       }
 *     });
 *   };
 *
 *   return (
 *     <button onClick={() => handleCreate(data)} disabled={createOU.isPending}>
 *       {createOU.isPending ? 'Creating...' : 'Create'}
 *     </button>
 *   );
 * }
 * ```
 */
export default function useCreateOrganizationUnit(): UseMutationResult<
  OrganizationUnit,
  Error,
  CreateOrganizationUnitRequest
> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('organizationUnits');
  const {showToast} = useToast();

  return useMutation<OrganizationUnit, Error, CreateOrganizationUnitRequest>({
    mutationFn: async (data: CreateOrganizationUnitRequest): Promise<OrganizationUnit> => {
      const serverUrl: string = getServerUrl();
      const response: {
        data: OrganizationUnit;
      } = await http.request({
        url: `${serverUrl}/organization-units`,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        data: data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: () => {
      // Invalidate and refetch organization units list after successful creation
      queryClient.invalidateQueries({queryKey: [OrganizationUnitQueryKeys.ORGANIZATION_UNITS]}).catch(() => {
        // Ignore invalidation errors
      });
      // Invalidate child OUs cache so tree view picks up the new child
      queryClient.invalidateQueries({queryKey: [OrganizationUnitQueryKeys.CHILD_ORGANIZATION_UNITS]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('create.success'), 'success');
    },
    onError: () => {
      showToast(t('create.error'), 'error');
    },
  });
}
