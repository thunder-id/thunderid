// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import AuthorizationDetailTypeQueryKeys from '../constants/authorization-detail-type-query-keys';

export default function useDeleteAuthorizationDetailType(): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();
  const {t} = useTranslation();
  const {showToast} = useToast();

  return useMutation<void, Error, string>({
    mutationFn: async (id: string): Promise<void> => {
      const serverUrl = getServerUrl();
      await http.request({
        url: `${serverUrl}/authorization-detail-types/${id}`,
        method: 'DELETE',
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: (_result, id) => {
      queryClient.removeQueries({queryKey: [AuthorizationDetailTypeQueryKeys.AUTHORIZATION_DETAIL_TYPE, id]});
      void queryClient.invalidateQueries({queryKey: [AuthorizationDetailTypeQueryKeys.AUTHORIZATION_DETAIL_TYPES]});
      showToast(t('authorizationDetailTypes:delete.success', 'Intent deleted successfully.'), 'success');
    },
    // A type that is already gone, or changed, shows as it is now once the list is fetched again.
    onError: () => {
      void queryClient.invalidateQueries({queryKey: [AuthorizationDetailTypeQueryKeys.AUTHORIZATION_DETAIL_TYPES]});
    },
  });
}
