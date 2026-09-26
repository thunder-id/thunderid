// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {ApplicationQueryKeys} from '@thunderid/configure-applications';
import {useAdministrationActions, useConfig, useToast, type AdministrationHttpLike} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import {deleteApplicationNatively} from '../utils/applicationAdministrationNative';

/**
 * Custom React hook to delete an application from the server.
 *
 * This hook uses TanStack Query mutations to handle the application deletion process,
 * providing loading states and error handling. Upon successful deletion, it automatically
 * removes the application from cache and invalidates the applications list query to
 * trigger a refetch.
 *
 * How the deletion is carried out is supplied by the console rather than chosen here. A Data Plane
 * console installs an action that runs the configured administration flow, which revokes the
 * application's tokens and detaches its sessions before the record is removed. A Control Plane
 * console installs none, and the deletion is `DELETE /applications/{id}`: that plane serves no
 * runtime, so there is nothing to revoke first.
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
  const {deleteApplication} = useAdministrationActions();

  return useMutation<void, Error, string>({
    mutationFn: async (applicationId: string): Promise<void> => {
      const client = http as unknown as AdministrationHttpLike;
      const serverUrl: string = getServerUrl();

      if (deleteApplication) {
        await deleteApplication(client, serverUrl, applicationId);

        return;
      }

      await deleteApplicationNatively(client, serverUrl, applicationId);
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
