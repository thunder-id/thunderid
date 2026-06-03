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
import UserTypeQueryKeys from '../constants/userTypeQueryKeys';

/**
 * Custom React hook to delete a user type from the server.
 *
 * @returns TanStack Query mutation object for deleting user types
 */
export default function useDeleteUserType(): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('userTypes');
  const {showToast} = useToast();

  return useMutation<void, Error, string>({
    mutationFn: async (userTypeId: string): Promise<void> => {
      const serverUrl: string = getServerUrl();
      await http.request({
        url: `${serverUrl}/user-types/${userTypeId}`,
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: (_data, userTypeId) => {
      queryClient.removeQueries({queryKey: [UserTypeQueryKeys.USER_TYPE, userTypeId]});
      queryClient.invalidateQueries({queryKey: [UserTypeQueryKeys.USER_TYPES]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('delete.success'), 'success');
    },
    onError: () => {
      showToast(t('delete.error'), 'error');
    },
  });
}
