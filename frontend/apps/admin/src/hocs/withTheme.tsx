// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {getCspNonce} from '@thunderid/design';
import {useThunderID} from '@thunderid/react';
import {createAppSdkThemes, createAppTheme, useGetAppConfig} from '@thunderid/system-apps';
import {createOxygenTheme, OxygenUIThemeProvider, HighContrastTheme} from '@wso2/oxygen-ui';
import {useMemo, type JSX, type ComponentType} from 'react';
import Head from '../components/Head';
import SdkTheme from '../components/SdkTheme';

/**
 * Applies the App theme the Console saved for the Admin App, to both the app shell and the SDK
 * components, on top of the app's own look, which it shows until the configuration loads or when
 * no App theme is saved. Its sign-in screens take the Gate theme of its application instead.
 */
export default function withTheme<P extends object>(WrappedComponent: ComponentType<P>) {
  return function WithTheme(props: P): JSX.Element {
    const {config} = useConfig();
    const {isSignedIn} = useThunderID();
    const {data: appConfig} = useGetAppConfig('ADMIN', {enabled: isSignedIn});
    const savedTheme = appConfig?.appTheme;
    const themeKey: string = JSON.stringify(savedTheme ?? null);
    const appTheme = useMemo(() => createAppTheme(savedTheme), [savedTheme]);
    const sdkThemes = useMemo(() => createAppSdkThemes(savedTheme), [savedTheme]);

    return (
      <OxygenUIThemeProvider
        // Re-mount when the saved theme arrives, so the provider picks up the new default theme.
        key={themeKey}
        nonce={getCspNonce()}
        themes={[
          {key: 'highContrast', label: 'High Contrast Theme', theme: HighContrastTheme},
          {key: 'default', label: 'Default Theme', theme: appTheme},
          ...(config?.brand?.design?.themes?.map((theme) => ({
            key: theme.key,
            label: theme.label,
            theme: typeof theme.theme === 'string' ? theme.theme : createOxygenTheme(theme.theme),
          })) ?? []),
        ]}
        initialTheme={config?.brand?.design?.initialTheme ?? 'default'}
      >
        <Head />
        <SdkTheme themes={sdkThemes}>
          <WrappedComponent {...props} />
        </SdkTheme>
      </OxygenUIThemeProvider>
    );
  };
}
