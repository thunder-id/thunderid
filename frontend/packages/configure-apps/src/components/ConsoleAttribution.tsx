// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Attribution, ThemeProvider} from '@thunderid/react';
import {useColorScheme} from '@wso2/oxygen-ui';
import type {JSX} from 'react';

/**
 * The SDK's attribution, drawn in the Console's current color scheme so it stays readable on light
 * and dark surfaces.
 */
export default function ConsoleAttribution(): JSX.Element {
  const {mode, systemMode} = useColorScheme();
  const scheme: 'dark' | 'light' = (mode === 'system' ? systemMode : mode) === 'dark' ? 'dark' : 'light';

  return (
    <ThemeProvider mode={scheme}>
      <Attribution />
    </ThemeProvider>
  );
}
