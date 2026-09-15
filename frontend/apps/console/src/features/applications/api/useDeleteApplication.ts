// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {ApplicationQueryKeys} from '@thunderid/configure-applications';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import {deleteApplicationViaFlow, type HttpLike} from '../utils/applicationAdministrationFlow';

/**
 * Custom React hook to delete an application from the server.
 *
 * This hook uses TanStack Query mutations to handle the application deletion process,
 * providing loading states and error handling. Upon successful deletion, it automatically
 * removes the application from cache and invalidates the applications list query to
 * trigger a refetch.
 *
 * The deletion runs through the configured administration flow, which revokes the application's
 * tokens and detaches its sessions before the record is removed. It falls back to
 * `DELETE /applications/{id}` when no such flow is configured.
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
      await deleteApplicationViaFlow(http as unknown as HttpLike, getServerUrl(), applicationId);
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
  });
}
