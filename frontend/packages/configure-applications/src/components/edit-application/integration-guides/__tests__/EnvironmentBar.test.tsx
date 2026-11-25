// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, within} from '@testing-library/react';
import type {RuntimeEnvironment} from '@thunderid/contexts';
import {MemoryRouter} from 'react-router';
import {describe, expect, it, vi} from 'vitest';
import EnvironmentBar from '../EnvironmentBar';

const development: RuntimeEnvironment = {id: 'env-dev', name: 'Development', runtimeUrl: 'https://dev.example.com'};
const production: RuntimeEnvironment = {id: 'env-prod', name: 'Production', runtimeUrl: 'https://prod.example.com'};

const openPicker = () => {
  fireEvent.mouseDown(screen.getByRole('combobox', {name: 'Environment'}));
  return within(screen.getByRole('listbox'));
};

describe('EnvironmentBar', () => {
  it('marks no environment and links no pipeline when it is not told of either', () => {
    render(
      <MemoryRouter>
        <EnvironmentBar environments={[development, production]} selectedEnvironment={development} onSelect={vi.fn()} />
      </MemoryRouter>,
    );

    expect(openPicker().getByRole('option', {name: /^Production/})).not.toHaveTextContent('Not');
    expect(screen.queryByRole('link', {name: /Deployment pipeline/})).not.toBeInTheDocument();
  });

  it('says an application not in the first environment is not deployed, and one not further on is not promoted', () => {
    render(
      <MemoryRouter>
        <EnvironmentBar
          environments={[development, production]}
          selectedEnvironment={development}
          onSelect={vi.fn()}
          notDeployedIds={['env-dev', 'env-prod']}
        />
      </MemoryRouter>,
    );

    const options = openPicker();
    expect(options.getByRole('option', {name: /^Development/})).toHaveTextContent('Not deployed to Development yet.');
    expect(options.getByRole('option', {name: /^Production/})).toHaveTextContent('Not promoted to Production yet.');
  });
});
