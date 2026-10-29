// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, waitFor} from '@testing-library/react';
import {ThemeProvider, useTheme as useSdkTheme} from '@thunderid/react';
import {OxygenUIThemeProvider} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {describe, expect, it} from 'vitest';
import SdkTheme from '../components/SdkTheme';
import AdminConsoleTheme from '../themes/AdminConsoleTheme';

function Probe(): JSX.Element {
  const {theme} = useSdkTheme();

  return <span data-testid="primary">{theme.vars.colors.primary.main}</span>;
}

describe('SdkTheme', () => {
  it('keeps its own CSS variables on the page over the SDK theme above it', async () => {
    const {getByTestId} = render(
      <ThemeProvider mode="light" theme={{colors: {primary: {main: '#ff8800'}}}}>
        <OxygenUIThemeProvider theme={AdminConsoleTheme}>
          <SdkTheme>
            <Probe />
          </SdkTheme>
        </OxygenUIThemeProvider>
      </ThemeProvider>,
    );

    const variable: string = /var\((--[^,)]+)/.exec(getByTestId('primary').textContent ?? '')![1];

    await waitFor(() =>
      expect(document.documentElement.style.getPropertyValue(variable).trim().toLowerCase()).toBe('#a31515'),
    );
  });
});
