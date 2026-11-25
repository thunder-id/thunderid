// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import AuthorizationDetailTypeQueryKeys from '../constants/authorization-detail-type-query-keys';
import type {AuthorizationDetailType, AuthorizationDetailTypeRequest} from '../models/authorization-detail-type';

export interface UpdateAuthorizationDetailTypeVariables {
  id: string;
  data: AuthorizationDetailTypeRequest;
}

export default function useUpdateAuthorizationDetailType(): UseMutationResult<
  AuthorizationDetailType,
  Error,
  UpdateAuthorizationDetailTypeVariables
> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();
  const {t} = useTranslation();
  const {showToast} = useToast();

  return useMutation<AuthorizationDetailType, Error, UpdateAuthorizationDetailTypeVariables>({
    mutationFn: async ({id, data}: UpdateAuthorizationDetailTypeVariables): Promise<AuthorizationDetailType> => {
      const serverUrl = getServerUrl();
      const response: {data: AuthorizationDetailType} = await http.request({
        url: `${serverUrl}/authorization-detail-types/${id}`,
        method: 'PUT',
        data,
      } as unknown as Parameters<typeof http.request>[0]);
      return response.data;
    },
    onSuccess: (result, {id}) => {
      // The saved type is the latest, so an open page shows it at once rather than the one before. The
      // query shares no structure with the type before, so a reorder is kept as the server returned it.
      queryClient.setQueryData([AuthorizationDetailTypeQueryKeys.AUTHORIZATION_DETAIL_TYPE, id], result);
      void queryClient.invalidateQueries({queryKey: [AuthorizationDetailTypeQueryKeys.AUTHORIZATION_DETAIL_TYPES]});
      showToast(t('authorizationDetailTypes:update.success', 'Intent updated successfully.'), 'success');
    },
  });
}
