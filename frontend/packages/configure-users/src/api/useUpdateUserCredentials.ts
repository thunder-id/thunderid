/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {useMutation, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';

export interface UpdateUserCredentialsVariables {
  userId: string;
  data: {
    credentials: Record<string, string>;
  };
}

export default function useUpdateUserCredentials(): UseMutationResult<void, Error, UpdateUserCredentialsVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {t} = useTranslation('users');
  const {showToast} = useToast();

  return useMutation<void, Error, UpdateUserCredentialsVariables>({
    mutationFn: async ({userId, data}: UpdateUserCredentialsVariables): Promise<void> => {
      const serverUrl: string = getServerUrl();

      await http.request({
        url: `${serverUrl}/users/${userId}/update-credentials`,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        data: data,
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: () => {
      showToast(t('updateCredentials.success'), 'success');
    },
    onError: () => {
      showToast(t('updateCredentials.error'), 'error');
    },
  });
}
