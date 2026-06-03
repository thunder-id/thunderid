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

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import OrganizationUnitQueryKeys from '../constants/organization-unit-query-keys';

/**
 * Custom React hook to delete an organization unit from the server.
 *
 * This hook uses TanStack Query mutations to handle the deletion process,
 * providing loading states and error handling. Upon successful deletion, it automatically
 * removes the organization unit from cache and invalidates the list query to trigger a refetch.
 *
 * @returns TanStack Query mutation object for deleting organization units
 *
 * @example
 * ```tsx
 * function DeleteOUButton({ id }: { id: string }) {
 *   const deleteOU = useDeleteOrganizationUnit();
 *
 *   const handleDelete = () => {
 *     if (confirm('Are you sure you want to delete this organization unit?')) {
 *       deleteOU.mutate(id, {
 *         onSuccess: () => {
 *           console.log('Organization unit deleted successfully');
 *         },
 *         onError: (error) => {
 *           console.error('Failed to delete organization unit:', error);
 *         }
 *       });
 *     }
 *   };
 *
 *   return (
 *     <button onClick={handleDelete} disabled={deleteOU.isPending}>
 *       {deleteOU.isPending ? 'Deleting...' : 'Delete'}
 *     </button>
 *   );
 * }
 * ```
 */
export default function useDeleteOrganizationUnit(): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('organizationUnits');
  const {showToast} = useToast();

  return useMutation<void, Error, string>({
    mutationFn: async (id: string): Promise<void> => {
      const serverUrl: string = getServerUrl();
      await http.request({
        url: `${serverUrl}/organization-units/${id}`,
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: (_data, id) => {
      // Remove the specific organization unit from cache
      queryClient.removeQueries({queryKey: [OrganizationUnitQueryKeys.ORGANIZATION_UNIT, id]});
      // Invalidate and refetch organization units list
      queryClient.invalidateQueries({queryKey: [OrganizationUnitQueryKeys.ORGANIZATION_UNITS]}).catch(() => {
        // Ignore invalidation errors
      });
      // Invalidate child OUs cache so tree view reflects the deletion
      queryClient.invalidateQueries({queryKey: [OrganizationUnitQueryKeys.CHILD_ORGANIZATION_UNITS]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('delete.success'), 'success');
    },
    onError: () => {
      showToast(t('delete.error'), 'error');
    },
  });
}
