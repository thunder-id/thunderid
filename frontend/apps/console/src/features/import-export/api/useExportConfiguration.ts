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
import type {ExportRequest, JSONExportResponse} from '../models/export-configuration';

/**
 * Custom React hook to export Product resource configurations as JSON.
 *
 * This hook uses TanStack Query's useMutation to handle the export operation.
 * The export API returns a JSON response containing an array of files along with
 * export metadata and summary information.
 *
 * @returns TanStack Query mutation result object with mutate function, loading state, and error information
 *
 * @example
 * ```tsx
 * function ExportButton() {
 *   const { mutate, isPending, error } = useExportConfiguration();
 *
 *   const handleExport = () => {
 *     mutate(
 *       {
 *         applications: ["*"], // Export all applications
 *       },
 *       {
 *         onSuccess: (data) => {
 *           console.log(`Exported ${data.summary.totalFiles} files`);
 *           // Process exported files...
 *         },
 *         onError: (error) => {
 *           console.error('Export failed:', error);
 *         },
 *       }
 *     );
 *   };
 *
 *   return (
 *     <button onClick={handleExport} disabled={isPending}>
 *       {isPending ? 'Exporting...' : 'Export Configuration'}
 *     </button>
 *   );
 * }
 * ```
 *
 * @public
 */
export default function useExportConfiguration(): UseMutationResult<JSONExportResponse, Error, ExportRequest> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useMutation<JSONExportResponse, Error, ExportRequest>({
    mutationFn: async (request: ExportRequest): Promise<JSONExportResponse> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: JSONExportResponse;
      } = await http.request({
        url: `${serverUrl}/export`,
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
