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
import {getErrorMessage} from '@thunderid/utils';
import {useTranslation} from 'react-i18next';
import ApplicationQueryKeys from '../constants/application-query-keys';

/**
 * Custom React hook to delete an application from the server.
 *
 * This hook uses TanStack Query mutations to handle the application deletion process,
 * providing loading states and error handling. Upon successful deletion, it automatically
 * removes the application from cache and invalidates the applications list query to
 * trigger a refetch.
 *
 * @returns TanStack Query mutation object for deleting applications with mutate function, loading state, and error information
 *
 * @example
 * ```tsx
 * function DeleteApplicationButton({ applicationId }: { applicationId: string }) {
 *   const deleteApp = useDeleteApplication();
 *
 *   const handleDelete = () => {
 *     if (confirm('Are you sure you want to delete this application?')) {
 *       deleteApp.mutate(applicationId, {
 *         onSuccess: () => {
 *           console.log('Application deleted successfully');
 *         },
 *         onError: (error) => {
 *           console.error('Failed to delete application:', error);
 *         }
 *       });
 *     }
 *   };
 *
 *   return (
 *     <button onClick={handleDelete} disabled={deleteApp.isPending}>
 *       {deleteApp.isPending ? 'Deleting...' : 'Delete Application'}
 *     </button>
 *   );
 * }
 * ```
 *
 * @public
 */
export default function useDeleteApplication(): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('applications');
  const {showToast} = useToast();

  return useMutation<void, Error, string>({
    mutationFn: async (applicationId: string): Promise<void> => {
      const serverUrl: string = getServerUrl();
      await http.request({
        url: `${serverUrl}/applications/${applicationId}`,
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: (_data, applicationId) => {
      // Remove the specific application from cache
      queryClient.removeQueries({queryKey: [ApplicationQueryKeys.APPLICATION, applicationId]});
      // Invalidate and refetch applications list
      queryClient.invalidateQueries({queryKey: [ApplicationQueryKeys.APPLICATIONS]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('delete.success'), 'success');
    },
    onError: (error) => {
      showToast(getErrorMessage(error, t, 'delete.error'), 'error');
    },
  });
}
