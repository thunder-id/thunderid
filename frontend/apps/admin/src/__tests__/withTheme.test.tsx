// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import type {JSX, ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import withTheme from '../hocs/withTheme';

const mockState: {enabled: boolean | undefined; signedIn: boolean; theme: Record<string, unknown> | undefined} = {
  enabled: undefined,
  signedIn: true,
  theme: undefined,
};

vi.mock('@thunderid/system-apps', async (importOriginal) => ({
  ...(await importOriginal<Record<string, unknown>>()),
  useGetAppConfig: (_type: string, options: {enabled?: boolean}) => {
    mockState.enabled = options.enabled;
    return {data: mockState.theme ? {appTheme: mockState.theme} : undefined};
  },
}));

vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({config: {brand: {favicon: {dark: 'dark.ico', light: 'light.ico'}}}}),
}));

vi.mock('@thunderid/react', () => ({
  ThemeProvider: ({children, mode, theme}: {children: ReactNode; mode: string; theme: unknown}): JSX.Element => (
    <div data-testid="sdk-theme" data-mode={mode} data-theme={JSON.stringify(theme ?? null)}>
      {children}
    </div>
  ),
  useThunderID: () => ({isSignedIn: mockState.signedIn}),
}));

vi.mock('../components/Head', () => ({default: (): null => null}));

function App(): JSX.Element {
  return <p>app</p>;
}

const Themed = withTheme(App);

describe('withTheme', () => {
  beforeEach(() => {
    mockState.enabled = undefined;
    mockState.signedIn = true;
    mockState.theme = undefined;
  });

  it("gives the SDK components the saved App theme over the app's own look", () => {
    mockState.theme = {borderRadius: 0, colorSchemes: {light: {primary: '#0f766e'}}};

    render(<Themed />);

    const sdkTheme: string = screen.getByTestId('sdk-theme').getAttribute('data-theme') ?? '';
    expect(screen.getByText('app')).toBeTruthy();
    expect(sdkTheme).toContain('#0f766e');
    expect(sdkTheme).toContain('"medium":"0px"');
    expect(sdkTheme).toContain('#f5f5f7');
  });

  it("waits for sign in before reading the configuration, and uses the Admin App's own look meanwhile", () => {
    mockState.signedIn = false;

    render(<Themed />);

    const sdkTheme: HTMLElement = screen.getByTestId('sdk-theme');
    expect(mockState.enabled).toBe(false);
    expect(sdkTheme.getAttribute('data-mode')).toBe('light');
    expect(sdkTheme.getAttribute('data-theme')).toContain('#3688FF');
  });
});
