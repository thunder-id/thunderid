// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import ApplicationQueryKeys from '../constants/application-query-keys';
import type {Application} from '../models/application';
import type {CreateApplicationRequest} from '../models/requests';

/**
 * Variables for the {@link useUpdateApplication} mutation.
 *
 * @public
 */
export interface UpdateApplicationVariables {
  /**
   * The unique identifier of the application to update
   */
  applicationId: string;
  /**
   * The updated application data
   */
  data: CreateApplicationRequest;
}

/**
 * Custom React hook to update an existing application in the server.
 *
 * This hook uses TanStack Query mutations to handle the application update process,
 * providing loading states and error handling. Upon successful update, it automatically
 * updates the cached application data and invalidates related queries to ensure
 * the UI reflects the latest changes.
 *
 * @returns TanStack Query mutation object for updating applications with mutate function, loading state, and error information
 *
 * @example
 * ```tsx
 * function UpdateApplicationForm({ applicationId }: { applicationId: string }) {
 *   const updateApp = useUpdateApplication();
 *
 *   const handleSubmit = (data: CreateApplicationRequest) => {
 *     updateApp.mutate(
 *       { applicationId, data },
 *       {
 *         onSuccess: (application) => {
 *           console.log('Application updated:', application);
 *         },
 *         onError: (error) => {
 *           console.error('Failed to update application:', error);
 *         }
 *       }
 *     );
 *   };
 *
 *   return (
 *     <button onClick={() => handleSubmit(data)} disabled={updateApp.isPending}>
 *       {updateApp.isPending ? 'Updating...' : 'Update Application'}
 *     </button>
 *   );
 * }
 * ```
 *
 * @public
 */
export default function useUpdateApplication(): UseMutationResult<Application, Error, UpdateApplicationVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('applications');
  const {showToast} = useToast();

  return useMutation<Application, Error, UpdateApplicationVariables>({
    mutationFn: async ({applicationId, data}: UpdateApplicationVariables): Promise<Application> => {
      const serverUrl: string = getServerUrl();
      const response: {
        data: Application;
      } = await http.request({
        url: `${serverUrl}/applications/${applicationId}`,
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        data: data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (_data, variables) => {
      // Invalidate and refetch the specific application
      queryClient
        .invalidateQueries({queryKey: [ApplicationQueryKeys.APPLICATION, variables.applicationId]})
        .catch(() => {
          // Ignore invalidation errors
        });
      // Invalidate and refetch applications list
      queryClient.invalidateQueries({queryKey: [ApplicationQueryKeys.APPLICATIONS]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('update.success'), 'success');
    },
  });
}
