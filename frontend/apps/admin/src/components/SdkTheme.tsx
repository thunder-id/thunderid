// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ThemeProvider, type ThemeProviderProps} from '@thunderid/react';
import type {AppColorScheme} from '@thunderid/system-apps';
import {useColorScheme} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';

/**
 * Themes the SDK components with the app's App theme, in whichever color scheme the app shows.
 */
export default function SdkTheme({
  children,
  themes,
}: {
  children: ReactNode;
  themes: Record<AppColorScheme, NonNullable<ThemeProviderProps['theme']>>;
}): JSX.Element {
  const {mode, systemMode} = useColorScheme();
  const scheme: AppColorScheme = (mode === 'system' ? systemMode : mode) === 'dark' ? 'dark' : 'light';

  return (
    <ThemeProvider mode={scheme} theme={themes[scheme]}>
      {children}
    </ThemeProvider>
  );
}
