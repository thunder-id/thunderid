// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import AuthorizationDetailTypeQueryKeys from '../constants/authorization-detail-type-query-keys';
import type {AuthorizationDetailType, AuthorizationDetailTypeRequest} from '../models/authorization-detail-type';

export default function useCreateAuthorizationDetailType(): UseMutationResult<
  AuthorizationDetailType,
  Error,
  AuthorizationDetailTypeRequest
> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();
  const {t} = useTranslation();
  const {showToast} = useToast();

  return useMutation<AuthorizationDetailType, Error, AuthorizationDetailTypeRequest>({
    mutationFn: async (data: AuthorizationDetailTypeRequest): Promise<AuthorizationDetailType> => {
      const serverUrl = getServerUrl();
      const response: {data: AuthorizationDetailType} = await http.request({
        url: `${serverUrl}/authorization-detail-types`,
        method: 'POST',
        data,
      } as unknown as Parameters<typeof http.request>[0]);
      return response.data;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({queryKey: [AuthorizationDetailTypeQueryKeys.AUTHORIZATION_DETAIL_TYPES]});
      showToast(t('authorizationDetailTypes:create.success', 'Intent created successfully.'), 'success');
    },
  });
}
