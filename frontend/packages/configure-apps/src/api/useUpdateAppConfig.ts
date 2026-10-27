// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {AppConfigQueryKeys, appConfigPath} from '@thunderid/system-apps';
import type {AppConfig} from '@thunderid/system-apps';
import {useTranslation} from 'react-i18next';
import type {BuilderAppConfig} from '../models/builder';

/**
 * Saves an app's configuration in its server-config section. Data planes pick it up through the usual
 * configuration sync.
 *
 * @returns TanStack Query mutation object. Call `mutate(config)` to save.
 *
 * @public
 */
export default function useUpdateAppConfig(): UseMutationResult<AppConfig, Error, BuilderAppConfig> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('apps');
  const {showToast} = useToast();

  return useMutation<AppConfig, Error, BuilderAppConfig>({
    mutationFn: async (config: BuilderAppConfig): Promise<AppConfig> => {
      // Only the app's own configuration: the sign-in application and its Gate design are saved on
      // the application.
      const payload: Pick<AppConfig, 'appTheme' | 'branding' | 'content' | 'navigation' | 'pages'> = {
        appTheme: config.appTheme,
        branding: config.branding,
        content: config.content,
        navigation: config.navigation,
        pages: config.pages,
      };
      const response: {data: {merged: AppConfig}} = await http.request({
        url: `${getServerUrl()}${appConfigPath(config.type)}`,
        method: 'PUT',
        headers: {'Content-Type': 'application/json'},
        data: payload,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data.merged;
    },
    onSuccess: (saved: AppConfig) => {
      queryClient.setQueryData([AppConfigQueryKeys.APP_CONFIG, saved.type], saved);
      showToast(
        saved.type === 'ACCOUNT'
          ? t('builder.account.saved', 'Account App saved.')
          : t('builder.admin.saved', 'Admin App saved.'),
        'success',
      );
    },
  });
}
