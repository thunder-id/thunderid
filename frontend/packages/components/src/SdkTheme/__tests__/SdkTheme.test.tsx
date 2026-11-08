// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, waitFor} from '@testing-library/react';
import {DefaultTheme} from '@thunderid/design';
import {ThemeProvider} from '@thunderid/react';
import {OxygenUIThemeProvider} from '@wso2/oxygen-ui';
import {describe, expect, it} from 'vitest';
import SdkTheme from '../SdkTheme';

/**
 * The values of the primary color variables the SDK theme sets on the page.
 */
const primaryValues = (): string[] =>
  Array.from(document.documentElement.style)
    .filter((property: string) => property.startsWith('--') && property.includes('primary'))
    .map((property: string) => document.documentElement.style.getPropertyValue(property).trim().toLowerCase());

describe('SdkTheme', () => {
  it('keeps its own CSS variables on the page over the SDK theme above it', async () => {
    render(
      <ThemeProvider mode="light" theme={{colors: {primary: {main: '#ff8800'}}}}>
        <OxygenUIThemeProvider theme={DefaultTheme}>
          <SdkTheme>
            <span>content</span>
          </SdkTheme>
        </OxygenUIThemeProvider>
      </ThemeProvider>,
    );

    // The console's own primary color wins over the one the SDK theme above set.
    await waitFor(() => expect(primaryValues()).toContain('#3688ff'));
    expect(primaryValues()).not.toContain('#ff8800');
  });
});
