// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import BuilderQueryKeys from '../constants/builder-query-keys';
import type {AppGateDesign} from '../models/builder';
import toGateDesign from '../utils/toGateDesign';

export interface UpdateGateDesignVariables {
  applicationId: string;
  gate: AppGateDesign;
}

/**
 * Saves the Gate design of the application an app signs in with. The application is read and
 * written back whole, with only its theme and layout changed.
 *
 * @returns TanStack Query mutation object.
 *
 * @public
 */
export default function useUpdateGateDesign(): UseMutationResult<AppGateDesign, Error, UpdateGateDesignVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();

  return useMutation<AppGateDesign, Error, UpdateGateDesignVariables>({
    mutationFn: async ({applicationId, gate}: UpdateGateDesignVariables): Promise<AppGateDesign> => {
      const url = `${getServerUrl()}/applications/${applicationId}`;
      const current: {data: Record<string, unknown>} = await http.request({
        url,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);
      // The certificate is read only; the server refuses it on update.
      const {certificate, ...application} = current.data;
      void certificate;

      const response: {data: AppGateDesign} = await http.request({
        url,
        method: 'PUT',
        headers: {'Content-Type': 'application/json'},
        data: {...application, layoutId: gate.layoutId ?? '', themeId: gate.themeId ?? ''},
      } as unknown as Parameters<typeof http.request>[0]);

      return toGateDesign(response.data);
    },
    onSuccess: (saved: AppGateDesign, {applicationId}: UpdateGateDesignVariables) => {
      queryClient.setQueryData([BuilderQueryKeys.SIGN_IN_APPLICATION, applicationId], saved);
    },
  });
}
