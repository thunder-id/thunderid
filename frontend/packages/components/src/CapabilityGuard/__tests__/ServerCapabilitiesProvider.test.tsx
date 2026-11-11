// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCapabilitiesContext} from '@thunderid/contexts';
import {render, screen} from '@thunderid/test-utils';
import type {JSX} from 'react';
import {describe, it, expect, vi} from 'vitest';
import ServerCapabilitiesProvider from '../ServerCapabilitiesProvider';

vi.mock('@thunderid/hooks', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/hooks')>()),
  useCapabilities: () => ({
    can: (_resource: string, action = 'view') => action === 'view',
    isLoading: false,
  }),
}));

function Probe(): JSX.Element {
  const {can} = useCapabilitiesContext();
  return <span>{`view:${String(can('app'))} delete:${String(can('app', 'delete'))}`}</span>;
}

describe('ServerCapabilitiesProvider', () => {
  it('hands the capabilities it reads to the pages beneath it', () => {
    render(
      <ServerCapabilitiesProvider>
        <Probe />
      </ServerCapabilitiesProvider>,
    );

    expect(screen.getByText('view:true delete:false')).toBeInTheDocument();
  });
});
