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

import {useThunderID} from '@thunderid/react';
import {useMutation, type UseMutationResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import type {ImportRequest, ImportResponse} from '../models/import-configuration';

/**
 * Custom React hook to import Product resource configurations.
 *
 * Supports both dry-run and actual import through the same endpoint.
 *
 * @public
 */
export default function useImportConfiguration(): UseMutationResult<ImportResponse, Error, ImportRequest> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useMutation<ImportResponse, Error, ImportRequest>({
    mutationFn: async (request: ImportRequest): Promise<ImportResponse> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: ImportResponse;
      } = await http.request({
        url: `${serverUrl}/import`,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        data: request,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
