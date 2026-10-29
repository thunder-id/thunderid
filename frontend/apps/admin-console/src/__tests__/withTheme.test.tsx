// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {render, screen} from '@testing-library/react';
import {useTheme} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import withDesign from '../hocs/withDesign';
import withTheme from '../hocs/withTheme';

const context: {isMetaLoading: boolean; meta: unknown} = {isMetaLoading: false, meta: null};

vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<Record<string, unknown>>()),
  useThunderID: () => context,
}));

vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({
    config: {brand: {product_name: 'ThunderID'}},
    getClientUuid: () => undefined,
    getServerUrl: () => 'https://localhost:8090',
  }),
}));

vi.mock('../components/Head', () => ({default: () => null}));

function Primary(): JSX.Element {
  return <span data-testid="primary">{useTheme().palette.primary.main}</span>;
}

const Themed = withDesign(withTheme(Primary));

const renderThemed = (): void => {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <Themed />
    </QueryClientProvider>,
  );
};

describe('withTheme', () => {
  beforeEach(() => {
    context.isMetaLoading = false;
    context.meta = null;
  });

  it('uses the Admin Console theme when the application has no Design', () => {
    renderThemed();

    expect(screen.getByTestId('primary')).toHaveTextContent('#a31515');
  });

  it('takes the Design theme of the application from the flow meta response', () => {
    context.meta = {
      design: {
        theme: {
          colorSchemes: {
            dark: {palette: {primary: {main: '#ff7300'}}},
            light: {palette: {primary: {main: '#ff7300'}}},
          },
        },
      },
    };
    renderThemed();

    expect(screen.getByTestId('primary')).toHaveTextContent('#ff7300');
  });

  it('waits for the flow meta response before rendering the app', () => {
    context.isMetaLoading = true;
    renderThemed();

    expect(screen.queryByTestId('primary')).not.toBeInTheDocument();
  });
});
