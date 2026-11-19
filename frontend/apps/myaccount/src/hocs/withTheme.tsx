// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {FontImporter, getCspNonce, getFontImportURL, useDesign, type Theme} from '@thunderid/design';
import {createAppTheme} from '@thunderid/system-apps';
import {Box, CircularProgress, createOxygenTheme, OxygenUIThemeProvider, HighContrastTheme} from '@wso2/oxygen-ui';
import type {JSX, ComponentType} from 'react';
import Head from '../components/Head';
import SdkTheme from '../components/SdkTheme';

/**
 * The app's own look: a soft grey page with white surfaces, which it shows when the application
 * it signs in with has no Design theme.
 */
const ACCOUNT_THEME = createAppTheme();

export default function withTheme<P extends object>(WrappedComponent: ComponentType<P>) {
  return function WithTheme(props: P): JSX.Element {
    const {config} = useConfig();
    // The Design theme of the signed-in application, when it has one, else the app's own.
    const {design, isLoading, theme} = useDesign(ACCOUNT_THEME as unknown as Theme);

    return (
      <OxygenUIThemeProvider
        nonce={getCspNonce()}
        themes={[
          {key: 'highContrast', label: 'High Contrast Theme', theme: HighContrastTheme},
          {key: 'default', label: 'Default Theme', theme: theme ?? ACCOUNT_THEME},
          ...(config?.brand?.design?.themes?.map((theme) => ({
            key: theme.key,
            label: theme.label,
            theme: typeof theme.theme === 'string' ? theme.theme : createOxygenTheme(theme.theme),
          })) ?? []),
        ]}
        initialTheme={config?.brand?.design?.initialTheme ?? 'default'}
      >
        <Head />
        <FontImporter importURL={getFontImportURL(design?.theme)} />
        {isLoading ? (
          <Box sx={{alignItems: 'center', display: 'flex', height: '100vh', justifyContent: 'center', width: '100vw'}}>
            <CircularProgress />
          </Box>
        ) : (
          <SdkTheme>
            <WrappedComponent {...props} />
          </SdkTheme>
        )}
      </OxygenUIThemeProvider>
    );
  };
}
