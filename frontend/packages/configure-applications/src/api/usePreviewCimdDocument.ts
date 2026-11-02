// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, type UseMutationResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import type {CimdPreview, CimdPreviewResponse} from '../models/cimd';
import toCimdPreview from '../utils/toCimdPreview';

/**
 * Retrieves and validates a Client ID Metadata Document through `POST /cimd/preview`, returning
 * what the administrator approves. Nothing is stored.
 *
 * @public
 */
export default function usePreviewCimdDocument(): UseMutationResult<CimdPreview, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useMutation<CimdPreview, Error, string>({
    mutationFn: async (clientId: string): Promise<CimdPreview> => {
      const response: {data: CimdPreviewResponse} = await http.request({
        url: `${getServerUrl()}/cimd/preview`,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        data: {clientId},
      } as unknown as Parameters<typeof http.request>[0]);

      return toCimdPreview(response.data);
    },
  });
}
